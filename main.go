package main

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	if err != nil {
		log.Println(err)
		serveError(w, r)
		return
	}
	logHTML, err := processRawLogHTML(rawLogHTML)
	if err != nil {
		log.Println("Failed to process /log HTML:", err)
		serveError(w, r)
		return
	}

	serveContents(w, r, bytes.NewReader(logHTML))
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
	http.HandleFunc("/log", serveLog)
	http.HandleFunc("/static/", serveStaticFile)
	http.HandleFunc("/assets/", serveStaticFile)
	http.HandleFunc("/.well-known/", serveStaticFile)
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
