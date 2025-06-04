package appserver

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"html/template"
	"io"
	"log"
	"net/http"
	"postmodernist1848.ru/repository/sqlite"
	"postmodernist1848.ru/resources"
	"strings"
	"sync"
	"time"
)

type metric struct {
	timestamp  time.Time
	statusCode int
	latency    time.Duration
}

type router struct {
	repository   *sqlite.Repository
	metricsMu    sync.RWMutex
	metricsQueue []metric
}

func newRouter(repository *sqlite.Repository) *router {
	return &router{
		repository:   repository,
		metricsQueue: make([]metric, 0),
	}
}

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

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func (s *router) MetricsAndLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		latency := time.Since(start)

		m := metric{timestamp: start, statusCode: rec.statusCode, latency: latency}

		s.metricsMu.Lock()
		defer s.metricsMu.Unlock()
		const queueSize = 1000
		s.metricsQueue = append(s.metricsQueue, m)
		if len(s.metricsQueue) > queueSize {
			s.metricsQueue = s.metricsQueue[len(s.metricsQueue)-queueSize:]
		}

		log.Printf("[%d] %s %s took %v", rec.statusCode, r.Method, r.URL.Path, latency)
	})
}

func (s *router) metricsHandler(w http.ResponseWriter, r *http.Request) {
	// render stats from metricsQueue as an HTML page
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()

	var totalRequests int
	var totalLatency time.Duration
	var errorCount int

	now := time.Now()
	for _, m := range s.metricsQueue {
		if now.Sub(m.timestamp) > time.Second {
			continue
		}
		totalRequests++
		totalLatency += m.latency
		if m.statusCode >= 400 {
			errorCount++
		}
	}
	averageLatency := time.Duration(int64(float64(totalLatency) / float64(totalRequests)))
	if totalRequests == 0 {
		averageLatency = 0
	}

	metricsHTML := &bytes.Buffer{}

	data := map[string]interface{}{
		"totalRequests":  totalRequests,
		"averageLatency": averageLatency,
		"errorCount":     errorCount,
		"errorRate":      float64(errorCount) / float64(totalRequests),
	}

	if err := resources.MetricsTemplate().Execute(metricsHTML, data); err != nil {
		log.Println("Failed to execute metrics template:", err)
		serveError(w, r)
		return
	}

	serveContents(w, r, metricsHTML)
}

func New(addr string, repository *sqlite.Repository) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, "index.html")
	})
	mux.HandleFunc("/{page}", contentsPageHandler)
	mux.HandleFunc("/{page}/", contentsPageHandler)
	mux.HandleFunc("/articles/{title}", articlesHandler)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, "assets/favicon.ico")
	})

	mux.HandleFunc("/static/", serveStaticFile)
	mux.HandleFunc("/assets/", serveStaticFile)

	r := newRouter(repository)
	mux.HandleFunc("/log", r.logHandler)
	mux.HandleFunc("GET /metrics", r.metricsHandler)

	mux.HandleFunc("GET /api/countlines/{username}", getCountLinesHandler)
	mux.HandleFunc("GET /api/message", r.getChatMessagesHandler)
	mux.HandleFunc("POST /api/message", r.postChatMessageHandler)
	mux.HandleFunc("GET /api/log", r.getLogHandler)
	mux.HandleFunc("PUT /api/log", r.putLogHandler)

	handler := r.MetricsAndLoggingMiddleware(mux)

	return &http.Server{Addr: addr, Handler: handler}
}
