package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	logger.Infof("Starting the application version=%s", version)

	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/rebuild", rebuildHandler)

	server := &http.Server{
		Addr:              cfg.LocalServerAddress,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Infof("HTTP server is listening on %s", cfg.LocalServerAddress)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	if err := rebuildCache(context.Background()); err != nil {
		logger.Errorf("Initial cache build failed, shutting down: %v", err)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			logger.Errorf("HTTP server Shutdown: %v", shutdownErr)
		}
		logger.Fatalf("Vault may be unreachable: %v", err)
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("Shutdown signal received")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Errorf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	<-idleConnsClosed
	logger.Info("Application has shut down gracefully")
	closeLogger()
}
