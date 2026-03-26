package main

import (
	"log"

	"os"

	"github.com/Rishabh-Kapri/pennywise/backend/workflows"
	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type Config struct {
	TEMPORAL_SERVER_HOST    string
	TEMPORAL_SERVER_PORT    string
}

func Load() Config {
	_ = godotenv.Load(".env")
	return Config{
		TEMPORAL_SERVER_HOST: os.Getenv("TEMPORAL_SERVER_HOST"),
		TEMPORAL_SERVER_PORT: os.Getenv("TEMPORAL_SERVER_PORT"),
	}
}

func main() {
	// 1. Connect to Temporal server
	config := Load()
	c, err := client.Dial(client.Options{
		HostPort: config.TEMPORAL_SERVER_HOST + ":" + config.TEMPORAL_SERVER_PORT,
	})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	// 2. Create a worker that listens on a task queue
	w := worker.New(c, "pennywise-tasks", worker.Options{})

	// 3. Register the workflow
	w.RegisterWorkflow(workflows.HelloWorldWorkflow)

	// 4. Start listening (blocks until interrupted)
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalf("Unable to start worker: %v", err)
	}
}