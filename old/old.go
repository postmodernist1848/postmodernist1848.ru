package old

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
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
	filepath := "old/contents/" + requestedPage
	content, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("Failed to read: `%s`", filepath)
		return nil, err
	}
	return content, nil
}

/* the default is getting a file path from map and
 * inserting its contents into the index template */
func ServeRoot(w http.ResponseWriter, r *http.Request) {
    
	contents, err := getContents(strings.TrimPrefix(r.URL.Path, "/old"))
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

/* log gets data from pastebin and inserts into the template
 * which adds some interactive elements with js
 */
func ServeLog(w http.ResponseWriter, r *http.Request) {
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

func ServeStaticFile(w http.ResponseWriter, r *http.Request) {
	// Extract the requested file path from the URL
	filePath := "." + strings.TrimPrefix(r.URL.Path, "/old")
	http.ServeFile(w, r, filePath)
}

