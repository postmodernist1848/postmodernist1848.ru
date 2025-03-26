package appserver

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"html/template"
	"io"
	"log"
	"net/http"
	"postmodernist1848.ru/appserver/api"
	"postmodernist1848.ru/repository/sqlite"
	"postmodernist1848.ru/resources"
	"strings"
)

func serveFile(w http.ResponseWriter, r *http.Request, name string) {
	http.ServeFileFS(w, r, resources.FS, name)
}

// serveStaticFile is a handler that serves a file from resources.FS
func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	serveFile(w, r, strings.TrimPrefix(r.URL.Path, "/"))
}

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
	logs, err := sqlite.GetNotes()
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

	mux.HandleFunc("/api/countlines/{username}", api.CountLinesHandler)

	mux.HandleFunc("GET /api/message", api.GETChatMessagesHandler)
	mux.HandleFunc("POST /api/message", api.POSTChatMessageHandler)
	mux.HandleFunc("GET /api/log", api.GETLogHandler)
	mux.HandleFunc("PUT /api/log", api.PUTLogHandler)

	return &http.Server{Addr: addr, Handler: mux}
}
