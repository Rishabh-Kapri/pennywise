package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/gmail"
	// "github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/pubsub"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

/*
* Main entry point for running the gmail api server
* Simple http handler apis
 */

func healthPage(w http.ResponseWriter, r *http.Request) {
	logger := logger.Logger(r.Context())

	requestBody, _ := json.Marshal(map[string]string{"status": "OK"})
	logger.Info("health check passed")
	w.WriteHeader(http.StatusOK)
	w.Write(requestBody)
	return
}

func watchHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logger.Logger(ctx)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("error reading request body", "error", err)
		err := errs.Wrap(errs.CodeInternalError, "error reading request body", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	var reqData gmail.GmailSyncRequest
	err = json.Unmarshal(data, &reqData)
	if err != nil {
		logger.Error("error unmarshalling request body", "error", err)
		err := errs.Wrap(errs.CodeInternalError, "error unmarshalling request body", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	res, err := gmail.NewService().WatchHandler(ctx, reqData)

	w.WriteHeader(http.StatusOK)
	resBytes, _ := json.Marshal(map[string]any{"historyID": res})
	w.Write(resBytes)
	return
}

func main() {
	logger.Setup("gmail-pubsub")
	config := config.LoadConfig()
	ctx := context.Background()
	log := logger.Logger(ctx)

	log.Info("pennywise-api", "url", config.PennywiseServiceURL+"/api")
	transport.CheckHealth(ctx, "pennywise-api", config.PennywiseServiceURL+"/api")
	// transport.CheckHealth(ctx, "mlp-api", config.MLPServiceURL+"/health")

	server := &http.Server{Addr: ":" + config.Port, Handler: nil}
	http.HandleFunc("/api", healthPage)
	http.HandleFunc("/api/watch", watchHandler)

	// go pubsub.PullMessages(ctx)

	go func() {
		log.Info("Server listening on port " + config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", "error", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := server.Shutdown(context.Background()); err != nil {
		logger.Fatal("server forced to shutdown", "error", err)
	}
}
