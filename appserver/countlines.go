package appserver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"postmodernist1848.ru/githublines"
	"sync/atomic"
)

const countlinesRequestsLimit = 10
const countlinesReposLimit = 50

var countlinesCurrentRequests atomic.Int32

func getCountLinesHandler(w http.ResponseWriter, r *http.Request) {
	for {
		current := countlinesCurrentRequests.Load()
		if current >= countlinesRequestsLimit {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, "Too many requests are being processed currently. Try again later.")
			return
		}
		if countlinesCurrentRequests.CompareAndSwap(current, current+1) {
			break
		}
	}
	defer countlinesCurrentRequests.Add(-1)

	username := r.PathValue("username")
	log.Printf("Handling countlines/ request. Username: %v", username)

	c, err := githublines.CountLines(r.Context(), username, countlinesReposLimit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		switch {
		case errors.Is(err, githublines.ErrGithubRequestFailed):
			fmt.Fprint(w, "Error: GitHub request failed")
		case errors.Is(err, githublines.ErrRepoLimitExceeded):
			fmt.Fprintf(w, "Error: Repository limit exceeded (%d)", countlinesReposLimit)
		default:
			fmt.Fprint(w, "Error: Internal server error")
		}
		return
	}

	totalCount := 0
	io.WriteString(w, "<ul>")
	for repo := range c {
		fmt.Fprintf(w, "<li>%v: %v lines</li>", repo.Name, repo.LineCount)
		totalCount += repo.LineCount
	}
	io.WriteString(w, "</ul>")
	fmt.Fprintf(w, "Total: %v lines", totalCount)
}
