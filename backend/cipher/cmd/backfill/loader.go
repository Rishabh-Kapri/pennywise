package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// loadPredictions loads prediction data either from a local JSON file or
// by fetching from the go-pennywise-api service.
func loadPredictions(
	ctx context.Context,
	pennywiseClient *transport.Client,
	budgetID uuid.UUID,
	dataPath string,
) []Prediction {
	log := logger.Logger(ctx)

	if dataPath != "" {
		log.Info("loading predictions from file", "path", dataPath)

		fileData, err := os.ReadFile(dataPath)
		if err != nil {
			logger.Fatal("Failed to read data file", "err", err)
		}

		var predictions []Prediction
		if err := json.Unmarshal(fileData, &predictions); err != nil {
			logger.Fatal("Failed to unmarshal data file", err)
		}
		return predictions
	}

	// Fetch predictions from go-pennywise-api via transport
	ctx = utils.WithBudgetID(ctx, budgetID)
	predictions, err := transport.Get[[]Prediction](ctx, pennywiseClient, "/api/predictions")
	if err != nil {
		logger.Fatal("Failed to fetch predictions", err)
	}
	return predictions
}
