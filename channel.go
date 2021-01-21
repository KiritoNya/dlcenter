package main

import (
	"fmt"
	"github.com/KiritoNya/database"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
	logger "github.com/sirupsen/logrus"
	"log"
	"time"
)

type Channel struct {
	Signaller chan string
}

/* CHANGE STATE

Add:
	inQueue -> download -> complete

Change state:

	DA		 ->	   A
	download -> pause		//Save progress of download (downloadedByte), close connection with server, kill gorutine.
	pause    -> download		//Read the progress from file, open connection with server, start gorutine.
	inQueue  ->	download	//Open connection with server, start gorutine, start download.
	complete -> terminate   //Delete item, delete request, delete response, delete from file.

	NB: terminate == deleted
	NB: completed call sizered program if item.SizeRed == true.

Remove:
	currentState -> terminated -> deleted

*/

func (c *Channel) activeNextDownload(id string) error {

	log.Println("ACTIVE:", q.Active)

	if q.Active != maxDownloads {
		var id string
		var rowNum int

		row, err := database.GetElements("SELECT COUNT(*) FROM Download WHERE Status='queued'")
		if err != nil {
			return err
		}
		row.Scan(&rowNum)

		log.Println("NUM:", rowNum)

		if rowNum > 0 {
			row, err = database.GetElementsWithValue("SELECT ID FROM Download WHERE Status=? ORDER BY ID LIMIT 1", inQueue.String())
			if err != nil {
				return err
			}
			row.Scan(&id)
			q.Items[id].Status = download
			c.Signaller <- id
		}

		//Find next queued item
		for i := 0; i < len(q.Items); i++ {
			pos := q.GetPosItem(id)
			id = q.GetIdByPos(pos + 1)
			if q.Items[id].Status == inQueue {
				logger.WithField("function", "HandlerState").Info("Start next download ", q.Items[id].NameFile)
				go c.ChangeStatus(download, id)
				break
			}
		}
	}
	return nil
}

func (c *Channel) ChangeStatus(status State, id string) {

	if q.checkIfIdExist(id) {

		q.Items[id].Status = status

		//Change status item in the database
		_, err := database.ChangeElement("UPDATE Download SET Status=? WHERE ID=?", q.Items[id].Status, id)
		if err != nil {
			logger.WithField("function", "ChangeStatus").Error(err)
			return
		}

		//Send signal
		c.Signaller <- id
	}
}

func (c *Channel) HandlerState() {

	for {
		select {
		case id := <-c.Signaller:
			switch q.Items[id].Status {
			case download:

				log.Println("Download:", q.Items[id].NameFile, q.Requests[id])
				logger.WithField("function", "HandlerState").Info(id+": ", download)

				//Start gorutine
				q.Items[id].DownloadedByte = 0

				//Increase active download number
				q.mu.Lock()
				q.Active++
				q.mu.Unlock()

				//Add Response
				if !q.Items[id].Torrent {

					q.AddResponse(id)

					//Run progress viewer
					go c.checkProgress(id)

				} else {

					if !CheckTorrent(q.Items[id].Hash) {

						log.Println("PASSO1")

						//Send torrent to the download torrent program
						err := q.Items[id].SendTorrent()
						if err != nil {
							log.Println(err)
						}

						log.Println("PASSO2")

						//Get hash file from download torrent program
						err = q.Items[id].GetHashTorrent()
						if err != nil {
							log.Println(err)
						}

						//Start check progress
						go c.checkProgressTorrent(id)

					} else {

						//Resume download
						err := q.Items[id].ResumeTorrent()
						if err != nil {
							logger.WithField("function", "HandlerState").Error("Error to resume torrent ", q.Items[id].NameFile)
						}

						//Start check progress
						go c.checkProgressTorrent(id)
					}
				}

			case pause:

				log.Println("Pause:", q.Items[id].NameFile, q.Response[id])
				logger.WithField("function", "HandlerState").Info(id+": ", pause)

				if !q.Items[id].Torrent {

					//Cancel download
					q.Response[id].Cancel()

					//Decrease active download number
					q.mu.Lock()
					q.Active--
					q.mu.Unlock()

					c.activeNextDownload(id)

				} else {
					err := q.Items[id].PauseTorrent()
					if err != nil {
						log.Println(err)
						logger.WithField("function", "HandlerState").Error("Error to pause torrent ", q.Items[id].NameFile)
					} else {

						//Decrease active download number
						q.mu.Lock()
						q.Active--
						q.mu.Unlock()

						c.activeNextDownload(id)

					}
				}

			case complete:

				log.Println("Complete:", q.Items[id].NameFile, q.Response[id])
				logger.WithField("function", "HandlerState").Info(id+": ", complete)

				_, err := database.ChangeElement("UPDATE Download SET Status=? WHERE ID=?", q.Items[id].Status, id)
				if err != nil {
					logger.WithField("function", "ChangeStatus").Error(err)
					return
				}

				q.Items[id].Percentage = "100.00%"
				q.Items[id].DownloadedByte = q.Items[id].Size

				//Decrease active download number
					q.mu.Lock()
					q.Active--
					q.mu.Unlock()

				c.activeNextDownload(id)

				//TODO: Send HTTP message at conversion program if sizered == true

				//Terminate manage in the removeItem function
			}
		}
	}
}

func (c *Channel) checkProgress(id string) {
	item := q.Items[id]
	resp := q.Response[id]
	t := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-t.C:
			item.DownloadedByte = resp.BytesComplete()
			item.Percentage = fmt.Sprintf("%.02f", resp.Progress()*100.0) + "%"
		case <-resp.Done:
			t.Stop()
			// check for errors
			if err := resp.Err(); err != nil {
				log.Println(err)
				return
			}
			item.Status = complete
			c.Signaller <- id
			return
		}
	}
}

func (c *Channel) checkProgressTorrent(id string) {

	item := q.Items[id]

	//Get Size
	err := item.SetSizeTorrent()
	if err != nil {
		log.Println(err)
		return
	}

	for range time.Tick(time.Second * 1) {

		if q.checkIfIdExist(id) {
			//Check state
			if q.Items[id].Status != download {
				return
			}
		}

		//Set downloaded byte
		err = item.SetDownloadedByteTorrent()
		if err != nil {
			log.Println(err)
			return
		}

		//Check finish
		if item.Size == item.DownloadedByte { //Completed
			item.Status = complete
			c.Signaller <- id
			return
		}

		//Check for error n/0
		if item.DownloadedByte != 0 {
			item.SetPercentage()
		}
	}
}
