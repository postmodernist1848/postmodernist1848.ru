package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"postmodernist1848.ru/domain"
	"postmodernist1848.ru/repository/sqlite"
)

func GETLogHandler(w http.ResponseWriter, r *http.Request) {
	notes, err := sqlite.GetNotes()
	if err != nil {
		log.Println("Could not get notes:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	JSON, err := json.Marshal(notes)
	if err != nil {
		log.Println("Could not marshal notes JSON:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(JSON)
}

func PUTLogHandler(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	token, err := os.ReadFile("api_token")
	if err != nil {
		log.Println("Could not read token file:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if username != "postmodernist1848" || string(token) != password {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var logs []domain.Note
	err = json.NewDecoder(r.Body).Decode(&logs)
	if err != nil {
		log.Println("Could not unmarshal logs JSON:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Println("Rewriting logs...")
	err = sqlite.RewriteNotes(logs)
	if err != nil {
		log.Println("Could not rewrite logs:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
