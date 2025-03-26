package server

import (
	"bytes"
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"postmodernist1848.ru/domain"
	"postmodernist1848.ru/githublines"
	"postmodernist1848.ru/repository/sqlite"
	"postmodernist1848.ru/resources"
	"strings"
)

// serveContents inserts reader data into contents template and moves
// <script>, <style>, <link>, <meta> tags into <head>
func serveContents(w http.ResponseWriter, r *http.Request, reader io.Reader) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// goquery automagically compiles the right tags into <head> here
	head, err := doc.Find("head").Html()
	if err != nil {
		log.Println("Could not render head", r.URL.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body, err := doc.Find("body").Html()
	if err != nil {
		log.Println("Could not render body", r.URL.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"head":     template.HTML(head),
		"contents": template.HTML(body),
	}
	err = resources.ContentsTemplate().Execute(w, data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Failed to execute template on %s", r.URL.Path)
	}
}

func serveError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	file, err := resources.Open("contents/error.html")
	if err != nil {
		log.Println("contents/error.html not found")
		return
	}
	serveContents(w, r, file)
}

func serveNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	file, err := resources.Open("contents/not-found.html")
	if err != nil {
		log.Println("contents/not-found.html not found")
		return
	}
	serveContents(w, r, file)
}

func serveContentsFromFile(w http.ResponseWriter, r *http.Request, path string) {
	file, err := resources.Open(path)
	if err != nil {
		log.Println(err)
		serveNotFound(w, r)
		return
	}
	serveContents(w, r, file)
}

func serveFile(w http.ResponseWriter, r *http.Request, name string) {
	http.ServeFileFS(w, r, resources.FS, name)
}

func contentsPageHandler(w http.ResponseWriter, r *http.Request) {
	serveContentsFromFile(w, r, "contents/"+r.PathValue("page")+".html")
}

func articlesHandler(w http.ResponseWriter, r *http.Request) {
	title := r.PathValue("title")
	file, err := resources.Open("articles/" + title + ".html")
	if err != nil {
		log.Println(err)
		serveNotFound(w, r)
		return
	}
	serveContents(w, r, file)
}

func logHandler(w http.ResponseWriter, r *http.Request) {
	logs, err := sqlite.GetLogs()
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

func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	serveFile(w, r, strings.TrimPrefix(r.URL.Path, "/"))
}

func getLogHandler(w http.ResponseWriter, r *http.Request) {
	logs, err := sqlite.GetLogs()
	if err != nil {
		log.Println("Could not get logs:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	JSON, err := json.Marshal(logs)
	if err != nil {
		log.Println("Could not marshal logs JSON:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(JSON)
}

func putLogHandler(w http.ResponseWriter, r *http.Request) {
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
	var logs []domain.Log
	err = json.NewDecoder(r.Body).Decode(&logs)
	if err != nil {
		log.Println("Could not unmarshal logs JSON:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Println("Rewriting logs...")
	err = sqlite.RewriteLogs(logs)
	if err != nil {
		log.Println("Could not rewrite logs:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func getChatMessagesHandler(w http.ResponseWriter, r *http.Request) {
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

func postChatMessageHandler(w http.ResponseWriter, r *http.Request) {
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
	getChatMessagesHandler(w, r)
}

func New(addr string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, "index.html")
	})
	mux.HandleFunc("/{page}", contentsPageHandler)
	mux.HandleFunc("/{page}/", contentsPageHandler)
	mux.HandleFunc("/articles/{title}", articlesHandler)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, "assets/favicon.ico")
		return
	})
	mux.HandleFunc("/log", logHandler)
	mux.HandleFunc("/static/", serveStaticFile)
	mux.HandleFunc("/assets/", serveStaticFile)

	// TODO use REST-like POST and GET semantics for messages
	mux.HandleFunc("GET /api/message", getChatMessagesHandler)
	mux.HandleFunc("POST /api/message", postChatMessageHandler)

	mux.HandleFunc("/api/countlines/", githublines.CountlinesHandler)
	mux.HandleFunc("GET /api/log", getLogHandler)
	mux.HandleFunc("PUT /api/log", putLogHandler)

	return &http.Server{Addr: addr, Handler: mux}
}
