package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"postmodernist1848.ru/appserver"
	"postmodernist1848.ru/repository/sqlite"
	"syscall"
	"time"
)

func main() {

	server := appserver.New(":80")

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs

		log.Println("Received signal:", sig)
		if err := sqlite.Database.Close(); err != nil {
			log.Println("Failed to close database:", err)
		}

		log.Println("Shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Println("Failed to close server:", err)
		}

		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		case syscall.SIGTERM:
			os.Exit(143)
		}
	}()

	log.Println(server.ListenAndServe())
}
