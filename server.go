package main

import (
	"encoding/json"
	"github.com/KiritoNya/database"
	qbt "github.com/KiritoNya/go-qbittorrent"
	logger "github.com/sirupsen/logrus"
	"log"
	"net/http"
	"os"
	"time"
)

const itemsDatabaseFile = "<username>:<password>tcp(<host_ip>:<port>)/<database>"
//const itemsDatabaseFile = "items.db"
const maxDownloads = 2
const driverDatabase = "mysql"
//const driverDatabase = "sqlite3"
const qbitorrentLink = "http://<host_ip>:<port>/"

var qb *qbt.Client

func init() {

	//Set logger
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	// Log as JSON instead of the default ASCII formatter.
	logger.SetFormatter(&logger.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logger.SetOutput(file)

	//Set database connection
	err = database.InitDB(itemsDatabaseFile, driverDatabase)
	if err != nil {
		panic(err)
	}

	//Set qbitorrent connection
	qb = qbt.NewClient(qbitorrentLink)

	isLog, err := qb.Login("admin", "Goghetto1106")
	if err != nil {
		panic(err)
	}

	if isLog {
		logger.WithField("function", "SendTorrent").Info("Logged")
	}

}

func main() {

	//Create channel
	var c Channel
	c.Signaller = make(chan string)

	//Start state handler
	go c.HandlerState()

	//Add to queue precedent downloads
	err := c.AddPrecedentDownloads()
	if err != nil {
		log.Fatalln(err)
	}

	//Create server
	mux := &http.ServeMux{}
	mux.HandleFunc("/change", c.Handle)
	mux.HandleFunc("/add", c.AddItem)
	mux.HandleFunc("/remove", c.RemoveItem)
	mux.HandleFunc("/show", q.ShowQueue)

	var handler http.Handler = mux
	handler = LogRequestHandler(handler)

	s := &http.Server{
		Addr:           ":8090",
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	logger.WithField("function", "Server").Info("Start server")

	s.ListenAndServe()
}

//TODO: Remove file
func (c *Channel) RemoveItem(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {

		ip := GetIP(r)

		//Get id params
		id, err := GetParams("id", r)
		if err != nil {
			logger.WithFields(logger.Fields{
				"function": "RemoveItem",
				"ip":       ip,
			}).Error(ip+":", err)
			PrintErr(w, err.Error())
			return
		}

		logger.WithFields(logger.Fields{
			"function": "RemoveItem",
			"ip":       ip,
		}).Info(id)

		if q.checkIfIdExist(id) {
			log.Println("[RemoveItem]", q.Items[id])

			log.Println("Terminate:", q.Items[id].NameFile, q.Requests[id])
			logger.WithFields(logger.Fields{
				"function": "RemoveItem",
				"ip":       ip,
			}).Info(id, ": ", terminate)

			//Delete item from database
			_, err = database.ChangeElement("DELETE FROM Download WHERE ID=?", id)
			if err != nil {
				logger.WithFields(logger.Fields{
					"function": "RemoveItem",
					"ip":       ip,
				}).Error(ip+": ", err)
				PrintInternalErr(w)
				return
			}

			var downloading bool

			if q.Items[id].Status == download {
				downloading = true
				if !q.Items[id].Torrent {

					//Abort
					err = q.Response[id].Cancel()
					if err != nil {
						PrintInternalErr(w)
					}

				}  else {

					//Remove torrent
					err = q.Items[id].RemoveTorrent()
					if err != nil {
						PrintInternalErr(w)
					}

				}
			}
			q.Items[id].Status = terminate

			if !q.Items[id].Torrent {
				//Delete item and relative signal done
				q.RemoveResponse(id)
				q.RemoveRequests(id)
			}
			q.Remove(id)

			if downloading {
				//Decrease active download number
				q.mu.Lock()
				q.Active--
				q.mu.Unlock()
				err = c.activeNextDownload(id)
				if err != nil {
					PrintInternalErr(w)
				}
			}

		} else {
			w.Write([]byte("<error> item not exist"))
		}
	} else {
		PrintErr(w, "")
	}
}

func (c *Channel) AddItem(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var resp ResponseAdd
		var i Item

		ip := GetIP(r)

		//Get item by JSON
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&i)
		if err != nil {
			logger.WithFields(logger.Fields{
				"function": "AddItem",
				"ip":       ip,
			}).Error(err)
			PrintErr(w, err.Error())
			return
		}

		//Check if exist
		for _, item := range q.Items {
			if item.Url == i.Url {
				logger.WithFields(logger.Fields{
					"function": "AddItem",
					"ip":       ip,
				}).Error("Item already present")
				PrintErr(w, "Item already present")
			}
		}

		//Check file path
		err = i.CheckFilePath()
		if err != nil {
			logger.WithFields(logger.Fields{
				"function": "AddItem",
				"ip":       ip,
			}).Error(err)
			PrintErr(w, err.Error())
			return
		}

		//Check URL
		if !i.Torrent {
			err = i.CheckUrl()
			if err != nil {
				PrintErr(w, err.Error())
			}
		}

		//Check maxDownloads
		if q.Active == maxDownloads {
			i.Status = inQueue
		} else {
			i.Status = download
		}

		//TODO: Verify if there is torrent value in item
		//TODO: Verify if there is sizeRed value in item

		logger.WithFields(logger.Fields{
			"function": "AddItem",
			"ip":       ip,
		}).Info(i.NameFile)

		//SetId
		id := GenerateID()

		//Set Name File
		if !i.Torrent {
			err = i.SetNameFile()
			if err != nil {
				PrintErr(w, err.Error())
			}
		} else {
			if i.NameFile == "" {
				err = i.SetNameFileTorrent()
				if err != nil {
					PrintErr(w, err.Error())
				}
			}
		}

		//Set Size
		if !i.Torrent{
			err = i.SetSize()
			if err != nil {
				PrintInternalErr(w)
				logger.WithFields(logger.Fields{
					"function": "AddItem",
					"ip":       ip,
				}).Error(err)
				return
			}
		}

		//Insert item into database
		_, err = database.ChangeElement("INSERT INTO Download(ID, Url, PathFile, Status, Torrent, SizeRed, Referer) VALUES(?, ?, ?, ?, ?, ?, ?)", id, i.Url, i.PathFile, i.Status, i.Torrent, i.SizeRed, i.Referer)
		if err != nil {
			PrintInternalErr(w)
			logger.WithFields(logger.Fields{
				"function": "AddItem",
				"ip":       ip,
			}).Error(err)
			return
		}

		//Add item to queue
		q.mu.Lock()
		q.Items[id] = &i
		q.mu.Unlock()

		//Add requests
		if !i.Torrent {
			q.AddRequest(id)
		}

		//Start download if q.Active /= maxDownloads
		if q.Items[id].Status == download {

			//Send signal
			c.Signaller <- id
		}

		//Create and do response
		resp.Item = i
		resp.Id = id

		b, err := json.MarshalIndent(resp, "", "\t")
		if err != nil {
			PrintInternalErr(w)
			logger.WithFields(logger.Fields{
				"function": "AddItem",
				"ip":       ip,
			}).Error(err)
			return
		}
		w.Write(b)

	} else {
		PrintErr(w, "")
	}
}

func (qu *Queue) ShowQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {

		enableCors(w)

		ip := GetIP(r)
		logger.WithFields(logger.Fields{
			"function": "ShowQueue",
			"ip":       ip,
		}).Info(ip, " - ", "Showed the queue")

		b, err := qu.GetItemsJson()
		if err != nil {
			logger.WithFields(logger.Fields{
				"function": "ShowQueue",
				"ip":       ip,
			}).Error(err)
			PrintInternalErr(w)
			return
		}

		w.Write(b)
	}
}

func (c *Channel) Handle(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {

		ip := GetIP(r)

		//Get status params
		stateString, err := GetParams("state", r)
		if err != nil {
			logger.WithFields(logger.Fields{
				"function": "Handle",
				"ip":       ip,
			}).Error(ip+":", err)
			PrintErr(w, "Wrong url params")
			return
		}
		logger.WithFields(logger.Fields{
			"function": "Handle",
			"ip":       ip,
		}).Info(stateString)

		//Get id params
		id, err := GetParams("id", r)
		if err != nil {
			logger.WithFields(logger.Fields{
				"function": "Handle",
				"ip":       ip,
			}).Error(ip+":", err)
			PrintErr(w, "Wrong url params")
			return
		}
		logger.WithFields(logger.Fields{
			"function": "Handle",
			"ip":       ip,
		}).Info(id)

		//Verify status
		if !checkIfStateExist(stateString) {
			logger.WithFields(logger.Fields{
				"function": "Handle",
				"ip":       ip,
			}).Error("State not found")
			PrintErr(w, "")
			return
		}

		//Verify ID
		if q.checkIfIdExist(id) {

			//Verify active download.
			switch State(stateString) {
			case download:
				log.Println("ACTIVE", q.Active)
				if q.Active != maxDownloads {
					q.Items[id].Status = download
				} else {
					w.Write([]byte("Too many items downloading"))
					return
				}
			case pause:
				q.Items[id].Status = pause
			}

			//Change status item in the database
			_, err = database.ChangeElement("UPDATE Download SET Status=? WHERE ID=?", q.Items[id].Status, id)
			if err != nil {
				logger.WithFields(logger.Fields{
					"function": "Handle",
					"ip":       ip,
				}).Error(err)
				return
			}

			//Send signal
			c.Signaller <- id

			logger.WithFields(logger.Fields{
				"function": "Handle",
				"ip":       ip,
			}).Info(q.Items[id].NameFile, " change state to ", q.Items[id].Status)

		} else {
			w.Write([]byte("<error> item not exist"))
		}

	} else {
		PrintErr(w, "")
	}
}
