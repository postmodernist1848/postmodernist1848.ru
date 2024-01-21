package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"postmodernist1848.ru/githublines"
	"strings"
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
}

func getContents(path string) []byte {
	requestedPage, ok := pathToFile[path]
	if !ok {
		log.Printf("Not in list: `%s`", path)
		return errorContents
	}
	filepath := "contents/" + requestedPage
	content, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("Failed to read: `%s`", filepath)
		content = errorContents
	}
	return content
}

func serveRoot(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"contents": template.HTML(getContents(r.URL.Path)),
	}
	err := indexTemplate.ExecuteTemplate(w, "index", data)
	if err != nil {
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
	}

	logHTML, err := processRawLogHTML(rawLogHTML)

	data := map[string]interface{}{
		"contents": template.HTML(logHTML),
	}
	err = indexTemplate.ExecuteTemplate(w, "index", data)
	if err != nil {
		log.Printf("Failed to execute template on %s", r.URL.Path)
	}
}

func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	// Extract the requested file path from the URL
	filePath := "." + r.URL.Path
	http.ServeFile(w, r, filePath)
}

func countLinesRepoResponse(w http.ResponseWriter, r *http.Request) {
	//TODO: use atomic counting to limit number of simultaneous requests

	username := strings.TrimPrefix(r.URL.Path, "/countlines/")
	log.Printf("Handling request. Username: %v", username)
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
		io.WriteString(w, fmt.Sprintf("<li>%v: %v lines</li>", repo.Name, repo.LineCount))
		totalCount += repo.LineCount
	}
	io.WriteString(w, "<ul>")
	io.WriteString(w, fmt.Sprintf("Total: %v lines", totalCount))
}

type ChatMessage struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

var messages []ChatMessage

func serveChatMessages(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<ul style=\"list-style: none\">"))
	for _, msg := range messages {
		w.Write([]byte("<li>"))
		w.Write([]byte(msg.Author))
		w.Write([]byte(": "))
		w.Write([]byte(msg.Text))
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
	messages = append(messages, msg)
}

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

	log.Println("listening for http on", http_port)
	go func() {
		log.Fatal(http.ListenAndServe(":"+http_port, nil))
	}()

	log.Println("listening for https on", https_port)
	log.Fatal(http.ListenAndServeTLS(":"+https_port, "server.crt", "server.key", nil))
}
