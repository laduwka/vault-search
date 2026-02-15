package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version string

func main() {
	logger.Infof("Starting the application version=%s", version)

	rebuildWg.Add(1)
	go func() {
		defer rebuildWg.Done()
		if err := rebuildCache(); err != nil {
			logger.Errorf("Initial cache build failed: %v", err)
		}
	}()

	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/rebuild", rebuildHandler)

	server := &http.Server{
		Addr:              cfg.LocalServerAddress,
		ReadHeaderTimeout: 10 * time.Second,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("Shutdown signal received")
		if err := server.Shutdown(context.Background()); err != nil {
			logger.Errorf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	logger.Infof("HTTP server is listening on %s", cfg.LocalServerAddress)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	<-idleConnsClosed
	logger.Info("Application has shut down gracefully")
}
