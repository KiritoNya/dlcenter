package main

import (
	"encoding/json"
	"fmt"
	"github.com/KiritoNya/database"
	"github.com/cavaliercoder/grab"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
	logger "github.com/sirupsen/logrus"
	"log"
	"os"
	"sort"
	"sync"
)


type Queue struct {
	Items map[string]*Item
	Requests map[string]*grab.Request
	Response map[string]*grab.Response
	Active int
	mu   sync.Mutex
}

type ResponseAdd struct {
	Item Item
	Id string
}

var q = NewQueue()

func NewQueue() Queue {
	var i Queue
	i.Items = make(map[string]*Item)
	i.Requests = make(map[string]*grab.Request)
	i.Response = make(map[string]*grab.Response)
	return i
}

func (q *Queue) Add(item *Item) {

	q.mu.Lock()
	id := GenerateID()
	q.Items[id] = item
	q.mu.Unlock()
	q.AddRequest(id)
	logger.WithField("function", "Add").Info("Added item ", q.Items[id].NameFile)
}

func (c *Channel) AddPrecedentDownloads() error {

	//Do query
	row, err := database.GetElements("SELECT * FROM Download")
	if err != nil {
		logger.WithField("function", "AddPrecedentDownload").Error("Error to do query")
	}

	//Fetch results
	for row.Next() { // Iterate and fetch the records from result cursor
		var itemDatabase struct {
			id string
			item Item
		}
		row.Scan(&itemDatabase.id, &itemDatabase.item.Url, &itemDatabase.item.Referer, &itemDatabase.item.PathFile, &itemDatabase.item.Status, &itemDatabase.item.Torrent, &itemDatabase.item.SizeRed)
		/*row2, err := database.GetElements("SELECT Referer FROM Download WHERE ID=" + itemDatabase.id)
		if err != nil {
			logger.WithField("function", "AddPrecedentDownload").Error("Error to do query")
		}
		for row2.Scan(&itemDatabase.item.Referer)*/
		log.Println(itemDatabase)
		q.Items[itemDatabase.id] = &itemDatabase.item
		//TODO: Calculate size of item
		//TODO: Calculate downloaded byte
		//TODO: Calculate NameFile
	}
	row.Close()

	logger.WithField("function", "AddPrecedentDownload").Info("Precedent download added")

	//Start downloads
	maxdl := maxDownloads
	for id, item := range q.Items {

		if !item.Torrent {
			q.AddRequest(id)
		}

		//Check PathFile
		err = item.CheckFilePath()
		if err != nil {
			logger.WithField("function", "AddPrecedentDownloads").Error(err)
			return err
		}

		if !item.Torrent {
			err = item.CheckUrl()
			if err != nil {
				logger.WithField("function", "AddPrecedentDownloads").Error(err)
				return err
			}
		}

		if !item.Torrent {

			//Set Size
			err = item.SetSize()
			if err != nil {
				logger.WithFields(logger.Fields{
					"function": "AddPrecedentDownloads",
				}).Error(err)
				return err
			}

		}

		if !item.Torrent {
			//Set NameFile
			item.SetNameFile()
		} else {
			item.SetNameFileTorrent()
		}

		log.Println("ITEM", item)

		//Set Status
		if item.Status == download || item.Status == inQueue {
			if maxdl > 0 {
				if item.Status != download {
					q.Items[id].Status = download
					_, err = database.ChangeElement("UPDATE Download SET Status=? WHERE ID=?", q.Items[id].Status, id)
					if err != nil {
						return err
					}
				}
				c.Signaller <- id
				maxdl--
			} else {
				if item.Status != inQueue {
					q.Items[id].Status = inQueue
					_, err = database.ChangeElement("UPDATE Download SET Status=? WHERE ID=?", q.Items[id].Status, id)
					if err != nil {
						return err
					}
				}
			}
		}

		if item.Status == complete {
			item.DownloadedByte = item.Size
			item.Percentage = "100.00%"
		}

		if item.Status == pause {
			if !item.Torrent {
				f, err := os.Open(item.PathFile)
				if err != nil {
					//TODO: In error state
					return err
				}

				fi, err := f.Stat()
				if err != nil {
					//TODO: In error state
					return err
				}
				item.DownloadedByte = fi.Size()
				item.Percentage = fmt.Sprintf("%.02f", 100.0/ float64(item.Size/item.DownloadedByte)) + "%"
			} else {

				err := item.SetSizeTorrent()
				if err != nil {
					logger.WithField("function", "AddPrecedentDownloads").Error(err)
				}

				err = item.SetDownloadedByteTorrent()
				if err != nil {
					logger.WithField("function", "AddPrecedentDownloads").Error(err)
				}

				item.SetPercentage()

			}
		}
	}

	logger.WithField("function", "AddPrecedentDownload").Info("Precedent download setted to download state")

	return nil
}

func (q *Queue) AddRequest(id string) {

	req, _ := grab.NewRequest(q.Items[id].PathFile, q.Items[id].Url)
	req.HTTPRequest.Header.Set("Referer", q.Items[id].Referer)

	q.mu.Lock()
	q.Requests[id] = req
	q.mu.Unlock()
	logger.WithField("function", "AddRequest").Info("Request ", id, " added")
}

func (q *Queue) AddResponse(id string) {
	client := grab.NewClient()
	q.mu.Lock()
	q.Response[id] = client.Do(q.Requests[id])
	q.mu.Unlock()
	logger.WithField("function", "AddResponse").Info("Response ", id, " added")
}

func (q *Queue) Remove(id string) {
	q.mu.Lock()
	delete(q.Items, id)
	q.mu.Unlock()
}

func (q *Queue) RemoveRequests(id string) {
	q.mu.Lock()
	delete(q.Requests, id)
	q.mu.Unlock()
	logger.WithField("function", "RemoveRequest").Info("Request ", id, " removed")
}

func (q *Queue) RemoveResponse(id string) {
	q.mu.Lock()
	delete(q.Response, id)
	q.mu.Unlock()
	logger.WithField("function", "RemoveResponse").Info("Response ", id, " removed")
}

func (q *Queue) GetItemsJson() ([]byte, error) {
	b, err := json.MarshalIndent(q.Items, "", "\t")
	return b, err
}

func (q *Queue) GetPosItem(id string) int {
	var keys []string
	count := 0
	for key, _ := range q.Items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == id {
			return count
		}
		count++
	}
	return -1
}

func (q *Queue) GetIdByPos(pos int) string {
	var keys []string
	count := 0
	for key, _ := range q.Items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if pos == len(q.Items) {
		pos = 0
	}
	for _, key := range keys {
		if count == pos {
			return key
		}
		count++
	}
	return ""
}

func (q *Queue) checkIfIdExist(id string) bool {
	for key, _ := range q.Items {
		if id == key {
			return true
		}
	}
	return false
}

func (q *Queue) GetIdByUrl(url string) string {
	for key, value := range q.Items {
		if value.Url == url {
			return key
		}
	}
	return ""
}
