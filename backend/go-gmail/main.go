package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gmail-transactions/pkg/config"
	"gmail-transactions/pkg/gmail"
	"gmail-transactions/pkg/logger"
	"gmail-transactions/pkg/pubsub"
)

func checkHealth(name string, url string) {
	for i := 1; i <= 5; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			slog.Info("health check passed", "service", name)
			return
		}
		if err != nil {
			slog.Warn("health check failed, retrying...", "service", name, "attempt", i, "error", err)
		} else {
			resp.Body.Close()
			slog.Warn("health check failed, retrying...", "service", name, "attempt", i, "status", resp.StatusCode)
		}
		time.Sleep(2 * time.Second)
	}
	slog.Error("health check failed after retries", "service", name)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusAccepted)

	data := gmail.Init()
	slog.Info("gmail init data", "data", data)
	requestBody, _ := json.Marshal(data)
	slog.Info("return data", "body", string(requestBody))
	w.Write(requestBody)
}

func main() {
	logger.Setup()
	cfg := config.LoadConfig()

	// Health check dependent services before starting
	checkHealth("pennywise-api", cfg.PennywiseApi+"/api")
	checkHealth("mlp-api", cfg.MLPApi+"/health")

	go pubsub.PullMessages()

	server := &http.Server{Addr: ":8080", Handler: nil}
	http.HandleFunc("/", handler)

	// Start server in a goroutine
	go func() {
		fmt.Println("Server listening on port 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", "error", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	if err := server.Shutdown(context.Background()); err != nil {
		logger.Fatal("server forced to shutdown", "error", err)
	}
	slog.Info("server exited")
}
