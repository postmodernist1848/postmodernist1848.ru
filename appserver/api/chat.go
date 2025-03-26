package api

import (
	"encoding/json"
	"html"
	"log"
	"net/http"
	"postmodernist1848.ru/domain"
	"postmodernist1848.ru/repository/sqlite"
)

func GETChatMessagesHandler(w http.ResponseWriter, r *http.Request) {
	msgs, err := sqlite.GetChatMessages()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for _, msg := range msgs {
		w.Write([]byte("<li>"))
		w.Write([]byte(html.EscapeString(msg.Author)))
		w.Write([]byte(": "))
		w.Write([]byte(html.EscapeString(msg.Text)))
		w.Write([]byte("</li>"))
	}
	w.Write([]byte("</ul>"))
}

func POSTChatMessageHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var msg domain.ChatMessage
	err := decoder.Decode(&msg)
	if err != nil {
		log.Println("failed to parse message: ", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(msg.Author) > 100 {
		log.Printf("Username too long (%v bytes)\n", len(msg.Author))
		http.Error(w, "Username too long", http.StatusBadRequest)
		return
	}
	if len(msg.Text) > 1848 {
		log.Printf("Message too long (%v bytes)\n", len(msg.Text))
		http.Error(w, "Message too long", http.StatusBadRequest)
		return
	}
	log.Println("Received message: ", msg)
	if err = sqlite.InsertChatMessage(msg); err != nil {
		log.Println("Failed to insert chat message: ", err)
		http.Error(w, "Failed to send chat message", http.StatusInternalServerError)
		return
	}
	GETChatMessagesHandler(w, r)
}
