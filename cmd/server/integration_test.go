package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"os"
	"postmodernist1848.ru/repository/sqlite"
	"slices"
	"testing"

	"postmodernist1848.ru/appserver"
	"postmodernist1848.ru/domain"
)

const testServerAddr = "localhost:8081"
const testServerURL = "http://" + testServerAddr

func testServer(t *testing.T) *http.Server {
	t.Helper()
	r, err := sqlite.Open("test.db")
	if err != nil {
		t.Fatal(err)
	}
	srv := appserver.New(testServerAddr, r)
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}()

	t.Cleanup(func() {
		t.Log("stopping server")
		if err := srv.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return srv
}

func httpGET(t *testing.T, path string) []byte {
	return httpRequest(t, path, http.MethodGet, nil, nil)
}

type auth struct {
	username, password string
}

func httpRequest(t *testing.T, path string, method string, body any, auth *auth) []byte {
	path = testServerURL + path

	var req *http.Request
	if body != nil {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err = http.NewRequest(
			method,
			path,
			bytes.NewReader(bodyJSON),
		)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(
			method,
			path,
			nil,
		)
	}

	if auth != nil {
		req.SetBasicAuth(auth.username, auth.password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("%s %s: got %d, want %d",
			method, path, resp.StatusCode, http.StatusOK)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return respBody
}

func RandStringBytes(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ      "
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func TestAllOk(t *testing.T) {
	t.Chdir("../..") // chdir to root of project to access resources
	testServer(t)

	t.Run("index", func(t *testing.T) {
		httpGET(t, "/")
	})

	t.Run("articles/", func(t *testing.T) {
		httpGET(t, "/articles/")
		articles := []string{"cfcracker", "haskell", "history", "ieee754", "manifesto"}
		for _, article := range articles {
			httpGET(t, "/articles/"+article)
		}
	})

	t.Run("contents", func(t *testing.T) {
		httpGET(t, "/about")
		httpGET(t, "/articles")
		httpGET(t, "/chat")
		httpGET(t, "/error")
		httpGET(t, "/fun")
		httpGET(t, "/game")
		httpGET(t, "/linalg")
		httpGET(t, "/not-found")
	})

	t.Run("chat", func(t *testing.T) {
		author := RandStringBytes(rand.Int()%10 + 4)
		text := RandStringBytes(rand.Int()%100 + 10)
		httpRequest(t, "/api/message", http.MethodPost, map[string]interface{}{
			"author": author,
			"text":   text,
		}, nil)
		res := httpGET(t, "/api/message")
		if !bytes.HasSuffix(res, []byte(fmt.Sprintf("<li>%s: %s</li></ul>", author, text))) {
			t.Log("expected new message, but got")
			t.Log(string(res))
			t.FailNow()
		}
	})

	t.Run("log", func(t *testing.T) {
		const testPassword = "password123"
		if err := os.WriteFile("api_token", []byte(testPassword), 0600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Remove("api_token"); err != nil {
				t.Error(err)
			}
		})
		var logs []domain.Note
		for i := range 20 {
			text := RandStringBytes(rand.Int()%100 + 10)
			logs = append(logs, domain.Note{fmt.Sprintf("%d.03.42", i+1), template.HTML(text)})
		}
		httpRequest(t, "/api/log", http.MethodPut, logs, &auth{"postmodernist1848", testPassword})
		res := httpGET(t, "/api/log")
		var resLogs []domain.Note
		if err := json.Unmarshal(res, &resLogs); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(logs, resLogs) {
			t.Log("logs not equal")
			t.Log("uploaded:")
			t.Log(logs)
			t.Log("received:")
			t.Log(resLogs)
			t.FailNow()
		}
	})
}
