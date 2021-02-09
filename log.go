package main

import (
	"encoding/json"
	"github.com/felixge/httpsnoop"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const connectionLogFile string = "connectionLog.json"
const logFile = "dlcenterLog.json"

type HTTPReqInfo struct {
	Method    string        `json:"method"`
	Url       string        `json:"url"`
	Referer   string        `json:"referer"`
	Ipaddr    string        `json:"ipaddr"`
	Code      int           `json:"code"` //Response Code 200, 400 ecc.
	Size      int64         `json:"size"` //Numero byte della risposta
	Duration  time.Duration `json:"duration"`
	Data      int64         `json:"data"`
	UserAgent string        `json:"userAgent"`
	muLogHTTP sync.Mutex
}

func (ri *HTTPReqInfo) logHTTPReq() {
	ri.muLogHTTP.Lock()
	out, err := json.MarshalIndent(ri, "", "  ")
	if err != nil {
		log.Println(err)
	}

	f, err := os.OpenFile(connectionLogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	if _, err = f.WriteString(string(out) + "\n"); err != nil {
		panic(err)
	}

	ri.muLogHTTP.Unlock()
}

func LogRequestHandler(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ri := &HTTPReqInfo{
			Method:    r.Method,
			Url:       r.URL.String(),
			Referer:   r.Header.Get("Referer"),
			UserAgent: r.Header.Get("User-Agent"),
			Data:      time.Now().Unix(),
		}

		ri.Ipaddr = GetIP(r)

		// this runs handler h and captures information about
		// HTTP request
		m := httpsnoop.CaptureMetrics(h, w, r)

		ri.Code = m.Code
		ri.Size = m.Written
		ri.Duration = m.Duration
		ri.logHTTPReq()
	}
	return http.HandlerFunc(fn)
}

