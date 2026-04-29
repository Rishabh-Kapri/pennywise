package main

import (
	"log/slog"
	"os"

	sharedLogger "github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	sharedTemporal "github.com/Rishabh-Kapri/pennywise/backend/shared/temporal"

	"github.com/Rishabh-Kapri/pennywise/backend/workflows/internal/workflow"

	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
	sdklog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

type Config struct {
	TEMPORAL_SERVER_HOST string
	TEMPORAL_SERVER_PORT string
}

func Load() Config {
	_ = godotenv.Load(".env")
	return Config{
		TEMPORAL_SERVER_HOST: os.Getenv("TEMPORAL_SERVER_HOST"),
		TEMPORAL_SERVER_PORT: os.Getenv("TEMPORAL_SERVER_PORT"),
	}
}

func main() {
	// 0. Set up shared logger
	sharedLogger.Setup("workflows-worker")

	// 1. Connect to Temporal server
	config := Load()
	c, err := client.Dial(client.Options{
		HostPort:           config.TEMPORAL_SERVER_HOST + ":" + config.TEMPORAL_SERVER_PORT,
		Logger:             sdklog.NewStructuredLogger(slog.Default()),
		ContextPropagators: sharedTemporal.ContextPropagators(),
	})
	if err != nil {
		sharedLogger.Fatal("Unable to create Temporal client", "error", err)
	}
	defer c.Close()

	// 2. Create a worker that listens on a task queue
	w := worker.New(c, sharedModel.PennywiseTaskQueue, worker.Options{
		UseBuildIDForVersioning: false,
	})

	// 3. Register workflows with explicit short names so callers don't need the full package path
	w.RegisterWorkflowWithOptions(workflow.EmailToTransactionWorkflow, sdkworkflow.RegisterOptions{
		Name: sharedModel.EmailToTransactionWorkflowName,
	})
	w.RegisterWorkflowWithOptions(workflow.ParsedEmailToTransactionWorkflow, sdkworkflow.RegisterOptions{
		Name: sharedModel.ParsedEmailToTransactionWorkflowName,
	})
	w.RegisterWorkflowWithOptions(workflow.RefreshGmailWatchWorkflow, sdkworkflow.RegisterOptions{
		Name: sharedModel.RefreshGmailWatchWorkflowName,
	})

	// 4. Start listening (blocks until interrupted)
	err = w.Run(worker.InterruptCh())
	if err != nil {
		sharedLogger.Fatal("Unable to start worker", "error", err)
	}
}
