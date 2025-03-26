package githublines

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type repository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
}

func countLinesFile(path string) (int, error) {
	r, err := os.Open(path)
	if err != nil {
		return 0, err
	}

	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return 0, err
		}
	}
}

func randomFilename() string {
	var randBytes = make([]byte, 16)
	_, _ = rand.Read(randBytes)
	return hex.EncodeToString(randBytes)
}

type RepoData struct {
	Name      string
	LineCount int
}

func CountLinesRepo(ctx context.Context, repo repository, c chan RepoData, wg *sync.WaitGroup) {
	defer wg.Done()
	url := "https://github.com/" + repo.FullName
	dir := "githublines/" + randomFilename()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, dir)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("GithubLines: git clone command error: %v", err)
		log.Print(string(out))
		c <- RepoData{repo.Name + " (error)", 0}
		return
	}
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			log.Println("os.RemoveAll failed: ", err)
		}
	}()

	var linesCount int
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if strings.HasSuffix(path, "/.git") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			_, matchedFile := codeFilenames[d.Name()]
			_, matchedExt := codeFiletypes[ext]
			if matchedFile || matchedExt {
				count, err := countLinesFile(path)
				if err != nil {
					log.Printf("countLinesFile error: %+v", err)
					return err
				}
				linesCount += count
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Walkdir error: %+v", err)
		c <- RepoData{repo.Name + " (error)", 0}
		return
	}
	c <- RepoData{repo.Name, linesCount}
	return
}

var (
	ErrGithubRequestFailed = errors.New("github request failed")
	ErrRepoLimitExceeded   = errors.New("repo limit exceeded")
)

// CountLines returns a generator of RepoData
func CountLines(ctx context.Context, username string, reposLimit int) (<-chan RepoData, error) {
	url := fmt.Sprintf("https://api.github.com/users/%v/repos", username)
	resp, err := http.Get(url)
	if err != nil {
		return nil, ErrGithubRequestFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("githublines: HTTP resp not OK: ", username, resp.StatusCode)
		} else {
			log.Println("githublines: HTTP resp not OK: ", username, resp.StatusCode, string(bodyBytes))
		}
		return nil, ErrGithubRequestFailed
	}
	var result []repository
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("GithubLines: Decoding error for", username, err)
		return nil, ErrGithubRequestFailed
	}
	if len(result) > reposLimit {
		return nil, ErrRepoLimitExceeded
	}
	c := make(chan RepoData)
	wg := sync.WaitGroup{}
	wg.Add(len(result))
	for _, repo := range result {
		go CountLinesRepo(ctx, repo, c, &wg)
	}
	go func() {
		wg.Wait()
		close(c)
	}()
	return c, nil
}
