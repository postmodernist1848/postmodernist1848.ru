package main

import (
	_ "embed"
	"html/template"
	"log"
	"net/http"
	"os"
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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", serveRoot)
	http.HandleFunc("/static/", serveStaticFile)
	http.HandleFunc("/assets/", serveStaticFile)

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
