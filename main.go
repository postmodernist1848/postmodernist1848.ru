package main

import (
	_ "embed"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
    "fmt"
    "encoding/json"
    "strings"
    "postmodernist1848.ru/githublines"
)

//go:embed index.html.tmpl
var indexTemplateString string
var indexTemplate = template.Must(template.New("index").Parse(indexTemplateString))
var errorContents = []byte("<h1>404: this page does not exist</h1>")

var pathToFile = map[string]string{
	"/":      "index.html",
	"/funi":  "funi.html",
	"/game":  "game.html",
	"/log":   "log.html",
	"/links": "links.html",
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

func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	// Extract the requested file path from the URL
	filePath := "." + r.URL.Path
	http.ServeFile(w, r, filePath)
}

func countLinesRepoResponse(w http.ResponseWriter, r *http.Request) {

    username := strings.TrimPrefix(r.URL.Path, "/countlines/")
    log.Printf("Handling request. Username: %v", username)
    url := fmt.Sprintf("https://api.github.com/users/%v/repos", username)
    resp, err := http.Get(url)
    if err != nil {
        log.Println("GithubLines: HTTP GET error for", username, err.Error())
        io.WriteString(w, "Failure getting data from Github")
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        log.Println("GithubLines: HTTP error for", username, err.Error())
        io.WriteString(w, "Failure getting data from Github")
        return
    }
    var result []githublines.Repo
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        log.Println("GithubLines: Decoding error for", username)
        io.WriteString(w, "Failure decoding data from Github")
        return
    }
    totalCount := 0;
    c := make(chan int)
    for _, repo := range result {
        go githublines.CountLinesRepo(repo, c)
    }
    io.WriteString(w, "<ul>")
    for _, repo := range result {
        linesCount := <-c
        io.WriteString(w, fmt.Sprintf("<li>%v: %v lines</li>", repo.Name, linesCount))
        totalCount += linesCount
    }
    io.WriteString(w, "<ul>")
    io.WriteString(w, fmt.Sprintf("Total: %v lines", totalCount))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", serveRoot)
	http.HandleFunc("/static/", serveStaticFile)
	http.HandleFunc("/assets/", serveStaticFile)
    http.HandleFunc("/countlines/", countLinesRepoResponse)

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
