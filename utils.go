package main

import (
	"errors"
	"github.com/segmentio/ksuid"
	"log"
	"net/http"
)

//Restituisce l'IP del client che ha effettuato la richiesta
func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

func PrintErr(w http.ResponseWriter, err string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte("400 - Bad Request!" + err + "\n"))
}

func PrintInternalErr(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 - Internal Server Error!\n"))
}

func GetParams(key string, r *http.Request) (string, error) {
	keys, err := r.URL.Query()[key]
	if !err || len(keys[0]) < 1 {
		return "", errors.New("Wrong url params")
	}
	return keys[0], nil
}

func GenerateID() string {
	return ksuid.New().String()
}

func CheckTorrent(hash string) bool {
	t, err := qb.Torrent(hash)
	if err != nil {
		log.Println("ECCOMI")
		return false
	}

	if t.TotalSize == 0 {
		return false
	}
	return true
}

//Funzione che abilita le Cors Policy
func enableCors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers:", "*")
}