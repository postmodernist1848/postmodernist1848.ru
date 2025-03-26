package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"postmodernist1848.ru/internal/server"
	"testing"
)

const testServerAddr = ":8080"

func testServer(t *testing.T) *http.Server {
	t.Helper()
	srv := server.New(testServerAddr)
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}()

	t.Cleanup(func() {
		t.Log("stopping server")
		if err := srv.Close(); err != nil {
			t.Error(err)
		}
	})
	return srv
}

func httpGET(t *testing.T, path string) []byte {
	resp, err := http.Get("http://localhost" + testServerAddr + path)
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%s: got %d, want %d", path, resp.StatusCode, http.StatusOK)
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	return bytes
}

func httpPOST(t *testing.T, path string, body any) []byte {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post("http://localhost"+testServerAddr+path, "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: got %d, want %d", path, resp.StatusCode, http.StatusOK)
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
		httpPOST(t, "/api/message", map[string]interface{}{
			"author": author,
			"text":   text,
		})
		res := httpGET(t, "/api/message")
		if !bytes.HasSuffix(res, []byte(fmt.Sprintf("<li>%s: %s</li></ul>", author, text))) {
			t.Log("expected new message, but got")
			t.Log(string(res))
			t.FailNow()
		}
	})
}
