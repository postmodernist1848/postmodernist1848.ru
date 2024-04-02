package main

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"postmodernist1848.ru/githublines"
)

//go:embed index.html.tmpl
var indexTemplateString string
var indexTemplate = template.Must(template.New("index").Parse(indexTemplateString))

//go:embed log.html.tmpl
var logTemplateString string
var logTemplate = template.Must(template.New("log").Parse(logTemplateString))
var errorContents = []byte("<h1>404: this page does not exist</h1>")

var pathToFile = map[string]string{
	"/":          "index.html",
	"/funi":      "funi.html",
	"/game":      "game.html",
	"/chat":      "chat.html",
	"/articles":  "articles.html",
	"/manifesto": "manifesto.html",
	"/haskell":   "haskell.html",
	"/links":     "links.html",
	"/linalg":    "linalg.html",
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

func serveRoot(w http.ResponseWriter, r *http.Request) {
    contents, err := getContents(r.URL.Path);
    if err != nil {
        contents = errorContents
        w.WriteHeader(http.StatusNotFound)
    }
	data := map[string]interface{}{
		"contents": template.HTML(contents),
	}
	err = indexTemplate.ExecuteTemplate(w, "index", data)
	if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Failed to execute template on %s", r.URL.Path)
	}
}

/* fetch the pastebin blog */
func getRawLogHTML() ([]byte, error) {
	const url = "https://pastebin.com/raw/vb43aqyz"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	text, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return text, nil
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
	rawLogHTML, err := getRawLogHTML()
	if err != nil {
		log.Println(err)
		w.Write(errorContents)
        w.WriteHeader(http.StatusServiceUnavailable)
	}

	logHTML, err := processRawLogHTML(rawLogHTML)
    if err != nil {
		log.Printf("Failed to process /log HTML")
        w.WriteHeader(http.StatusInternalServerError)
    }

	data := map[string]interface{}{
		"contents": template.HTML(logHTML),
	}
	err = indexTemplate.ExecuteTemplate(w, "index", data)
	if err != nil {
		log.Printf("Failed to execute template on %s", r.URL.Path)
        w.WriteHeader(http.StatusInternalServerError)
	}
}

func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	// Extract the requested file path from the URL
	filePath := "." + r.URL.Path
	http.ServeFile(w, r, filePath)
}

const countlines_requests_limit = 5
var countlines_current_requests atomic.Int32

func countLinesRepoResponse(w http.ResponseWriter, r *http.Request) {

    if countlines_current_requests.Load() >= countlines_requests_limit {
        io.WriteString(w, "Too many requests are being processed currently. Try later")
        return
    }

    countlines_current_requests.Add(1)
	username := strings.TrimPrefix(r.URL.Path, "/countlines/")
	log.Printf("Handling countlines/ request. Username: %v", username)
	url := fmt.Sprintf("https://api.github.com/users/%v/repos", username)
	resp, err := http.Get(url)
	if err != nil {
		log.Println("GithubLines: HTTP GET error for", username, err)
		io.WriteString(w, "Failure getting data from Github")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("GithubLines: HTTP resp not OK for", username, resp.StatusCode)
		} else {
			log.Println("GithubLines: HTTP resp not OK for", username, string(bodyBytes))
		}
		io.WriteString(w, "Failure getting data from Github")
		return
	}
	var result []githublines.Repo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("GithubLines: Decoding error for", username, err)
		io.WriteString(w, "Failure decoding data from Github")
		return
	}
	c := make(chan githublines.RepoData)
	for _, repo := range result {
		go githublines.CountLinesRepo(repo, c)
	}
	io.WriteString(w, "<ul>")
	totalCount := 0
	for range result {
		repo := <-c
		fmt.Fprintf(w, "<li>%v: %v lines</li>", repo.Name, repo.LineCount)
		totalCount += repo.LineCount
	}
	io.WriteString(w, "<ul>")
	fmt.Fprintf(w, "Total: %v lines", totalCount)

    countlines_current_requests.Add(-1)
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
		w.Write([]byte(author))
		w.Write([]byte(": "))
		w.Write([]byte(text))
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
		w.WriteHeader(400)
		return
	}
	log.Println("Received message: ", msg)
	insertChatMessage(database, msg)
}

func insertChatMessage(db *sql.DB, message ChatMessage) error {
	query := `INSERT INTO message(author, text) VALUES (?, ?)`
	statement, err := db.Prepare(query)
	if err != nil {
		return err
	}
	_, err = statement.Exec(message.Author, message.Text)
	return err
}

var database *sql.DB

func main() {
	http_port := "80"
	https_port := "443"

	http.HandleFunc("/", serveRoot)
	http.HandleFunc("/log", serveLog)
	http.HandleFunc("/api/chat-messages", serveChatMessages)
	http.HandleFunc("/api/send-message", chatSendHandler)
	http.HandleFunc("/static/", serveStaticFile)
	http.HandleFunc("/assets/", serveStaticFile)
	http.HandleFunc("/countlines/", countLinesRepoResponse)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		err := database.Close()
		if err != nil {
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

	log.Println("Listening for http on", http_port)
	go func() {
		log.Fatal(http.ListenAndServe(":"+http_port, nil))
	}()

	log.Println("Listening for https on", https_port)
	log.Fatal(http.ListenAndServeTLS(":"+https_port, "server.crt", "server.key", nil))
}
