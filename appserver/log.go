package appserver

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"postmodernist1848.ru/domain"
	"postmodernist1848.ru/resources"
)

func (s *router) getLogHandler(w http.ResponseWriter, _ *http.Request) {
	notes, err := s.repository.GetNotes()
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

func (s *router) putLogHandler(w http.ResponseWriter, r *http.Request) {
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
	err = s.repository.RewriteNotes(logs)
	if err != nil {
		log.Println("Could not rewrite logs:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *router) logHandler(w http.ResponseWriter, r *http.Request) {
	logs, err := s.repository.GetNotes()
	if err != nil {
		log.Println("Could not get logs:", err)
		serveError(w, r)
		return
	}

	logHTML := &bytes.Buffer{}
	if err = resources.LogTemplate().Execute(logHTML, logs); err != nil {
		log.Println("Failed to process /log HTML:", err)
		serveError(w, r)
		return
	}

	serveContents(w, r, logHTML)
}
