package appserver

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"html/template"
	"io"
	"log"
	"net/http"
	"postmodernist1848.ru/repository/sqlite"
	"postmodernist1848.ru/resources"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type metric struct {
	timestamp  time.Time
	statusCode int
	latency    time.Duration
}

type router struct {
	repository *sqlite.Repository

	totalRequests   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	errorResponses  *prometheus.CounterVec
}

func newRouter(repository *sqlite.Repository) *router {
	var (
		totalRequests = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests processed, by path and method.",
			},
			[]string{"path", "method"},
		)

		requestDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Histogram of HTTP request durations in seconds, by path and method.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"path", "method"},
		)

		// errorResponses counts HTTP errors (status codes >= 400), labeled by path, method, and status code
		errorResponses = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_responses_error_total",
				Help: "Total number of HTTP error responses (status >= 400), by path, method, and status code.",
			},
			[]string{"path", "method", "status"},
		)
	)
	return &router{
		repository:      repository,
		totalRequests:   totalRequests,
		requestDuration: requestDuration,
		errorResponses:  errorResponses,
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
			promhttp.Handler().ServeHTTP(w, r)
			return
		}

		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		latency := time.Since(start)

		s.totalRequests.WithLabelValues(r.URL.Path, r.Method).Inc()
		s.requestDuration.WithLabelValues(r.URL.Path, r.Method).Observe(latency.Seconds())
		if rec.statusCode >= 400 {
			s.errorResponses.WithLabelValues(r.URL.Path, r.Method, strconv.Itoa(rec.statusCode)).Inc()
		}

		log.Printf("[%d] %s %s took %v", rec.statusCode, r.Method, r.URL.Path, latency)
	})
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

	mux.HandleFunc("GET /api/countlines/{username}", getCountLinesHandler)
	mux.HandleFunc("GET /api/message", r.getChatMessagesHandler)
	mux.HandleFunc("POST /api/message", r.postChatMessageHandler)
	mux.HandleFunc("GET /api/log", r.getLogHandler)
	mux.HandleFunc("PUT /api/log", r.putLogHandler)

	handler := r.MetricsAndLoggingMiddleware(mux)

	return &http.Server{Addr: addr, Handler: handler}
}
