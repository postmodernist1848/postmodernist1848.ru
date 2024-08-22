package main

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"postmodernist1848.ru/githublines"
	"postmodernist1848.ru/old"
	"strings"
	"syscall"
)

//go:embed index.html.tmpl
var indexTemplateString string
var indexTemplate = template.Must(template.New("index").Parse(indexTemplateString))

//go:embed log.html.tmpl
var logTemplateString string
var logTemplate = template.Must(template.New("log").Parse(logTemplateString))

var pathToFile = map[string]string{
	"/fun":       "fun.html",
	"/game":      "game.html",
	"/chat":      "chat.html",
	"/articles/": "articles.html",
	"/about":     "about.html",
	"/linalg":    "linalg.html",
}

func getContents(path string) (*os.File, error) {
	requestedPage, ok := pathToFile[path]
	if !ok {
		log.Printf("Not in list: `%s`", path)
		return nil, fmt.Errorf("not in list: `%s`", path)
	}
	filepath := "contents/" + requestedPage
	return os.Open(filepath)
}

func serveContents(w http.ResponseWriter, r *http.Request, reader io.Reader) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var builder strings.Builder
	var headProcessingErrors []error = nil
	doc.Find("style, link, meta").Each(func(_ int, item *goquery.Selection) {
		HTML, err := goquery.OuterHtml(item)
		if err != nil {
			headProcessingErrors = append(headProcessingErrors, err)
		} else {
			builder.WriteString(HTML)
		}
		item.Remove()
	})
	if headProcessingErrors != nil {
		log.Println(headProcessingErrors)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// goquery add html, head and body tags if they are not present
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
		"head":     template.HTML(builder.String()),
		"contents": template.HTML(head + body),
	}
	err = indexTemplate.ExecuteTemplate(w, "index", data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Failed to execute template on %s", r.URL.Path)
	}
}

func serveNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	file, err := os.Open("contents/not-found.html")
	if err != nil {
		log.Println("contents/not-found.html not found")
		return
	}
	serveContents(w, r, file)
}

func serveError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	file, err := os.Open("contents/error.html")
	if err != nil {
		log.Println("contents/error.html not found")
		return
	}
	serveContents(w, r, file)
}

/* the default is getting a file path from map and
 * inserting its contents into the index template */
func serveRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "index.html")
		return
	}

	if r.URL.Path == "/favicon.ico" {
		http.ServeFile(w, r, "assets/favicon.ico")
		return
	}

	var file *os.File
	var err error
	if !strings.HasPrefix(r.URL.Path, "/articles/") || r.URL.Path == "/articles/" {
		file, err = getContents(r.URL.Path)
	} else {
		file, err = os.Open("." + r.URL.Path + ".html")
	}

	if err != nil {
		serveNotFound(w, r)
		return
	}
	serveContents(w, r, file)
}

type Log = struct {
	Date string        `json:"date"`
	HTML template.HTML `json:"html"`
}

func getLogs() ([]Log, error) {
	rows, err := database.Query(`SELECT date, html FROM note`)
	if err != nil {
		return nil, fmt.Errorf("failed to query note table: %w", err)
	}
	var logs []Log
	for rows.Next() {
		var date string
		var HTML string
		err = rows.Scan(&date, &HTML)
		if err != nil {
			return nil, fmt.Errorf("failed to read note table row: %w", err)
		}
		logs = append(logs, Log{date, template.HTML(HTML)})
	}
	return logs, nil
}

func serveLog(w http.ResponseWriter, r *http.Request) {

	logs, err := getLogs()
	if err != nil {
		log.Println("Could not get logs:", err)
		serveError(w, r)
		return
	}

	logHTML := &bytes.Buffer{}
	if err = logTemplate.Execute(logHTML, logs); err != nil {
		log.Println("Failed to process /log HTML:", err)
		serveError(w, r)
		return
	}

	serveContents(w, r, logHTML)
}

func rewriteLogs(logs []Log) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM note`)
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO note(date, html) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	for _, l := range logs {
		_, err = stmt.Exec(l.Date, l.HTML)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func logHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		logs, err := getLogs()
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
		w.Write(JSON)
	} else if r.Method == "PUT" {
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
		var logs []Log
		err = json.NewDecoder(r.Body).Decode(&logs)
		if err != nil {
			log.Println("Could not unmarshal logs JSON:", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Println("Rewriting logs...")
		err = rewriteLogs(logs)
		if err != nil {
			log.Println("Could not rewrite logs:", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
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

func chatMessagesHandler(w http.ResponseWriter, r *http.Request) {
	row, err := database.Query("SELECT * FROM message ORDER BY id")
	if err != nil {
		log.Println("Failed to retrieve messages: ", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	defer row.Close()
	w.Write([]byte("<ul style=\"list-style: none\">"))
	for row.Next() {
		var id int
		var author string
		var text string
		// FIXME: handle errors
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
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if l := len(msg.Text); l >= 1848 {
		log.Printf("Message too long (%v bytes)\n", l)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Println("Received message: ", msg)
	if err = insertChatMessage(msg); err != nil {
		log.Println("Failed to insert chat message: ", err)
	}
}

var database *sql.DB

func insertChatMessage(message ChatMessage) error {
	_, err := database.Exec(`INSERT INTO message(author, text) VALUES (?, ?)`,
		message.Author, message.Text)
	return err
}

func main() {

	http.HandleFunc("/", serveRoot)
	http.HandleFunc("/log", serveLog)
	http.HandleFunc("/static/", serveStaticFile)
	http.HandleFunc("/assets/", serveStaticFile)
	http.HandleFunc("/.well-known/", serveStaticFile)

	http.HandleFunc("/api/chat-messages", chatMessagesHandler)
	http.HandleFunc("/api/send-message", chatSendHandler)
	http.HandleFunc("/api/countlines/", githublines.CountlinesHandler)
	http.HandleFunc("/api/log", logHandler)

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
			os.Exit(1)
		}
		log.Println("Successfully closed the database")
		switch sig {
		case os.Interrupt:
			os.Exit(130)
		case syscall.SIGTERM:
			os.Exit(143)
		}
	}()

	var err error
	database, err = sql.Open("sqlite3", "database.db")
	if err != nil {
		log.Fatal("Failed to open sqlite database: ", err)
	}

	migrate()

	const httpPort = "80"
	log.Println("Listening on port", httpPort)
	log.Fatal(http.ListenAndServe(":"+httpPort, nil))
}
