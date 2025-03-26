package main

import (
	"errors"
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

func assertOK(t *testing.T, path string) {
	resp, err := http.Get("http://localhost" + testServerAddr + path)
	if err != nil {
		t.Error(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%s: got %d, want %d", path, resp.StatusCode, http.StatusOK)
	}
}

func TestAllOk(t *testing.T) {
	t.Chdir("../..") // chdir to root of project to access resources
	testServer(t)

	t.Run("index", func(t *testing.T) {
		assertOK(t, "/")
	})

	t.Run("articles/", func(t *testing.T) {
		assertOK(t, "/articles/")
		articles := []string{"cfcracker", "haskell", "history", "ieee754", "manifesto"}
		for _, article := range articles {
			assertOK(t, "/articles/"+article)
		}
	})

	t.Run("contents", func(t *testing.T) {
		assertOK(t, "/about")
		assertOK(t, "/articles")
		assertOK(t, "/chat")
		assertOK(t, "/error")
		assertOK(t, "/fun")
		assertOK(t, "/game")
		assertOK(t, "/linalg")
		assertOK(t, "/not-found")
	})
}
