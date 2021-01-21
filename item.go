package main

import (
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
)

type Item struct {
	Url            string
	Referer        string
	NameFile       string
	PathFile       string
	Size           int64
	DownloadedByte int64
	Percentage     string
	Status         State
	Torrent        bool
	SizeRed        bool
	Hash           string
	//TODO: DateAdd
	//TODO: DateComplete
}

func (i *Item) SetStatus(status State) {
	i.Status = status
}

func (i *Item) SetNameFile() error {

	name := path.Base(i.PathFile)
	if name == "/" {
		return errors.New("Wrong File Name. Path file is not a file, it is a directory.")
	}
	if name == "" {
		return errors.New("File name not setted.")
	}
	i.NameFile = path.Base(i.PathFile)
	return nil
}

func (i *Item) SetNameFileTorrent() error {

	link, err := url.Parse(i.Url)
	if err != nil {
		return err
	}

	q := link.Query()
	i.NameFile, err = url.PathUnescape(q["dn"][0])
	if err != nil {
		return err
	}
	return nil
}

func (i *Item) SetSize() error {

	var req *http.Request

	req, _ = http.NewRequest(http.MethodHead, i.Url, nil)
	if i.Referer != "" {
		req.Header.Set("Referer", i.Referer)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	contentLength := resp.Header.Get("content-length")
	i.Size, err = strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return err
	}
	return nil
}

func (i *Item) SetSizeTorrent() error {

	//Get hash if it is not setted
	if i.Hash == "" {
		err := i.GetHashTorrent()
		if err != nil {
			return err
		}
	}

	for {
		//Get Information of torrent
		t, err := qb.Torrent(i.Hash)
		if err != nil {
			return err
		}

		if t.TotalSize != -1 {
			i.Size = int64(t.TotalSize)
			break
		}
	}
	return nil
}

func (i *Item) SetPercentage() {
	i.Percentage = fmt.Sprintf("%.02f", 100.0/ float64(i.Size/i.DownloadedByte)) + "%"
}

func (i *Item) CheckFilePath() error {
	//TODO: Verify if is possible create directory
	return nil
}

func (i *Item) CheckUrl() error {
	req, err := http.NewRequest("HEAD", i.Url, nil)
	if err != nil {
		return err
	}

	if i.Referer != "" {
		req.Header.Set("Referer", i.Referer)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("Invalid url")
	}
	return nil
}

func (i *Item) SendTorrent() error {

	// were not using any filters so the options map is empty
	options := map[string]string{
		"savepath": path.Dir(i.PathFile),
	}

	resp, err := qb.DownloadFromLink(i.Url, options)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("error" + resp.Status)
	}

	content, _ := ioutil.ReadAll(resp.Body)

	log.Println(string(content))
	return nil
}

func (i *Item) GetHashTorrent() error {

	//Create map filters
	var filters map[string]string
	filters = make(map[string]string)

	// connect to qbittorrent client
	trList, err := qb.Torrents(filters)
	if err != nil {
		return err
	}

	//Check hash of torrent file
	for _, torrent := range trList {
		if torrent.Name == i.NameFile {
			i.Hash = torrent.Hash
			return nil
		}
	}

	return errors.New("Torrent file not found")
}

func (i *Item) SetDownloadedByteTorrent() error {

	//Get hash if it is not setted
	if i.Hash == "" {
		err := i.GetHashTorrent()
		if err != nil {
			return err
		}
	}

	//Get Information of torrent
	t, err := qb.Torrent(i.Hash)
	if err != nil {
		return err
	}

	i.DownloadedByte = int64(t.TotalDownloaded)
	return nil
}

func (i *Item) PauseTorrent() error {


	//Get hash if it is not setted
	if i.Hash == "" {
		err := i.GetHashTorrent()
		if err != nil {
			return err
		}
	}

	resp, err := qb.Pause(i.Hash)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("error" + resp.Status)
	}

	return nil
}

func (i *Item) ResumeTorrent() error {

	//Get hash if it is not setted
	if i.Hash == "" {
		err := i.GetHashTorrent()
		if err != nil {
			return err
		}
	}

	resp, err := qb.Resume(i.Hash)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("error" + resp.Status)
	}

	return nil
}

func (i *Item) RemoveTorrent() error {
	//Get hash if it is not setted
	if i.Hash == "" {
		err := i.GetHashTorrent()
		if err != nil {
			return err
		}
	}

	resp, err := qb.DeletePermanently([]string{i.Hash})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("error" + resp.Status)
	}

	content, _ := ioutil.ReadAll(resp.Body)

	log.Println(string(content))
	return nil
}
