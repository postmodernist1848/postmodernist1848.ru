package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"postmodernist1848.ru/appserver"
	"postmodernist1848.ru/repository/sqlite"
	"syscall"
	"time"
)

func main() {

	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		databasePath = "database.sqlite"
	}
	repository, err := sqlite.Open(databasePath)
	if err != nil {
		log.Fatal("Failed to open sqlite database: ", err)
	}

	server := appserver.New(":8080", repository)

	code := make(chan int)
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs

		log.Println("Received signal:", sig)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Println("Failed to close server:", err)
		}

		if err := repository.Close(); err != nil {
			log.Println("Failed to close database:", err)
		}

		switch sig {
		case syscall.SIGINT:
			code <- 130
		case syscall.SIGTERM:
			code <- 143
		}
	}()

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Println(err)
	}
	os.Exit(<-code)
}
