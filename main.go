package main

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
	"postmodernist1848.ru/githublines"
	"postmodernist1848.ru/old"
)

//go:embed index.html.tmpl
var indexTemplateString string
var indexTemplate = template.Must(template.New("index").Parse(indexTemplateString))

//go:embed log.html.tmpl
var logTemplateString string
var logTemplate = template.Must(template.New("log").Parse(logTemplateString))
var notFoundContents = []byte("<h1>404: this page does not exist</h1>")
var errorContents = []byte("<h1>Server error</h1>")

var pathToFile = map[string]string{
	"/fun":      "fun.html",
	"/game":     "game.html",
	"/chat":     "chat.html",
	"/articles": "articles.html",

	"/about":  "about.html",
	"/linalg": "linalg.html",
}

func getContents(path string) ([]byte, error) {
	requestedPage, ok := pathToFile[path]
	if !ok {
		log.Printf("Not in list: `%s`", path)
		return nil, errors.New(fmt.Sprintf("Not in list: `%s`", path))
	}
	filepath := "contents/" + requestedPage
	content, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("Failed to read: `%s`", filepath)
		return nil, err
	}
	return content, nil
}

func serveContents(w http.ResponseWriter, r *http.Request, contents []byte) {
	data := map[string]interface{}{
		"contents": template.HTML(contents),
	}
	err := indexTemplate.ExecuteTemplate(w, "index", data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Failed to execute template on %s", r.URL.Path)
	}
}

/* the default is getting a file path from map and
 * inserting its contents into the index template */
func serveRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "index.html")
		return
	}
	contents, err := getContents(r.URL.Path)
	if err != nil {
		contents = notFoundContents
		w.WriteHeader(http.StatusNotFound)
	}
	serveContents(w, r, contents)
}

func serveArticles(w http.ResponseWriter, r *http.Request) {
	contents, err := os.ReadFile("." + r.URL.Path + ".html")
	if err != nil {
		log.Println(err)
		contents = notFoundContents
		w.WriteHeader(http.StatusNotFound)
	}
	serveContents(w, r, contents)
}

/* insert into the log template */
func processRawLogHTML(rawHTML []byte) ([]byte, error) {
	var tpl bytes.Buffer
	data := map[string]interface{}{
		"contents": template.HTML(rawHTML),
	}
	if err := logTemplate.Execute(&tpl, data); err != nil {
		return nil, err
	}
	return tpl.Bytes(), nil
}

func serveLog(w http.ResponseWriter, r *http.Request) {
	rawLogHTML, err := os.ReadFile("log.html")
	var logHTML []byte
	if err != nil {
		log.Println(err)
		logHTML = errorContents
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		logHTML, err = processRawLogHTML(rawLogHTML)
		if err != nil {
			log.Printf("Failed to process /log HTML")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	serveContents(w, r, logHTML)
}

func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	// Extract the requested file path from the URL
	filePath := "." + r.URL.Path
	http.ServeFile(w, r, filePath)
}

type ChatMessage struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

func serveChatMessages(w http.ResponseWriter, r *http.Request) {
	row, err := database.Query("SELECT * FROM message ORDER BY id")
	if err != nil {
		log.Fatal(err)
	}
	defer row.Close()
	w.Write([]byte("<ul style=\"list-style: none\">"))
	for row.Next() {
		var id int
		var author string
		var text string
		row.Scan(&id, &author, &text)
		w.Write([]byte("<li>"))
		w.Write([]byte(html.EscapeString(author)))
		w.Write([]byte(": "))
		w.Write([]byte(html.EscapeString(text)))
		w.Write([]byte("</li>"))
	}
	w.Write([]byte("</ul>"))
}

func chatSendHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var msg ChatMessage
	err := decoder.Decode(&msg)
	if err != nil {
		log.Println("failed to parse message: ", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if l := utf8.RuneCountInString(msg.Text); l >= 1848 {
		log.Printf("Message too long (%v runes)\n", l)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Println("Received message: ", msg)
	insertChatMessage(msg)
}

var database *sql.DB

func insertChatMessage(message ChatMessage) error {
	query := `INSERT INTO message(author, text) VALUES (?, ?)`
	statement, err := database.Prepare(query)
	if err != nil {
		return err
	}
	_, err = statement.Exec(message.Author, message.Text)
	return err
}

func main() {
	httpPort := "80"
	httpsPort := "443"

	http.HandleFunc("/", serveRoot)
	http.HandleFunc("/articles/", serveArticles)
	http.HandleFunc("/log", serveLog)
	http.HandleFunc("/static/", serveStaticFile)
	http.HandleFunc("/assets/", serveStaticFile)
	http.HandleFunc("/api/chat-messages", serveChatMessages)
	http.HandleFunc("/api/send-message", chatSendHandler)
	http.HandleFunc("/api/countlines/", githublines.ServeCountlines)

	// old uses current /api
	http.HandleFunc("/old/", old.ServeRoot)
	http.HandleFunc("/old/log", old.ServeLog)
	http.HandleFunc("/old/static/", serveStaticFile)
	http.HandleFunc("/old/assets/", old.ServeStaticFile)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		if database.Close() != nil {
			log.Println("Failed to close database")
			os.Exit(137)
		}
		log.Println("Successfully closed the database")
		switch sig {
		case os.Interrupt:
			os.Exit(130)
		case syscall.SIGTERM:
			os.Exit(143)
		}
	}()

	log.Println("Opening database...")
	var err error
	database, err = sql.Open("sqlite3", "database.db")
	if err != nil {
		log.Fatal("Failed to open sqlite database: ", err)
	}

	log.Println("Listening for http on", httpPort)
	go func() {
		log.Fatal(http.ListenAndServe(":"+httpPort, nil))
	}()

	log.Println("Listening for https on", httpsPort)
	log.Fatal(http.ListenAndServeTLS(":"+httpsPort, "server.crt", "server.key", nil))
}
