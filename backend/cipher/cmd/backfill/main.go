package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"time"

	db "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/repository"

	"github.com/google/uuid"
)

type Prediction struct {
	ID                    string  `json:"id"`
	BudgetID              string  `json:"budgetId"`
	TransactionID         string  `json:"transactionId"`
	EmailText             string  `json:"emailText"`
	Amount                float64 `json:"amount"`
	Account               *string `json:"account"`
	Payee                 *string `json:"payee"`
	Category              *string `json:"category"`
	HasUserCorrected      *bool   `json:"hasUserCorrected"`
	UserCorrectedPayee    *string `json:"userCorrectedPayee"`
	UserCorrectedAccount  *string `json:"userCorrectedAccount"`
	UserCorrectedCategory *string `json:"userCorrectedCategory"`
}

func main() {
	var dataPath string
	flag.StringVar(&dataPath, "data", "", "path to json file containing prediction data")
	flag.Parse()

	cfg := config.Load()

	ctx := context.Background()
	log := logger.Logger(ctx)

	pennywiseAPI := os.Getenv("PENNYWISE_API")
	if pennywiseAPI == "" {
		logger.Fatal("PENNYWISE_API environment variable is required")
	}

	budgetIDStr := os.Getenv("BUDGET_ID")
	if budgetIDStr == "" {
		logger.Fatal("BUDGET_ID environment variable is required")
	}
	budgetID, err := uuid.Parse(budgetIDStr)
	if err != nil {
		logger.Fatal("Invalid BUDGET_ID: %v", err)
	}

	// authToken := os.Getenv("AUTH_TOKEN")
	// if authToken == "" {
	// 	log.Fatal("AUTH_TOKEN environment variable is required")
	// }

	dbConn, err := db.ConnectWithURL(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer dbConn.Close()

	// Ollama client via shared transport
	ollamaEngine := httpclient.NewHttpTransport(cfg.OllamaURL)
	ollamaTransport := transport.NewClient("ollama", ollamaEngine)
	ollamaClient := client.NewOllamaClient(ollamaTransport)

	// Pennywise API client via shared transport
	pennywiseEngine := httpclient.NewHttpTransport(pennywiseAPI)
	pennywiseClient := transport.NewClient("pennywise-api", pennywiseEngine)

	embeddingRepo := repository.NewTransactionEmbeddingRepository(dbConn)

	var predictions []Prediction
	var source string
	if dataPath != "" {
		log.Info("loading predictions from file", "path", dataPath)

		fileData, err := os.ReadFile(dataPath)
		if err != nil {
			logger.Fatal("Failed to read data file", "err", err)
		}
		if err := json.Unmarshal(fileData, &predictions); err != nil {
			logger.Fatal("Failed to unmarshal data file", err)
		}
		source = "historical"
	} else {
		// Fetch predictions from go-pennywise-api via transport
		ctx = utils.WithBudgetID(ctx, budgetID)
		var err error
		predictions, err = transport.Get[[]Prediction](ctx, pennywiseClient, "/api/predictions")
		if err != nil {
			logger.Fatal("Failed to fetch predictions", err)
		}
		source = "prediction"
	}

	log.Info("loaded predictions", "count", len(predictions))

	success, failed := 0, 0
	for i, p := range predictions {
		if p.EmailText == "" {
			log.Warn("skipping prediction with empty email text", "id", p.ID)
			failed++
			continue
		}
		transactionType := "debited"
		if p.Amount > 0 {
			transactionType = "credited"
		}
		cleanedEmailText := utils.CleanEmailText(p.EmailText, transactionType)
		// slog.Info("email text", "text", p.EmailText, "cleaned", cleanedEmailText)

		// Determine the correct labels (prefer user-corrected values)
		payee := deref(p.Payee)
		category := deref(p.Category)
		account := deref(p.Account)

		if p.HasUserCorrected != nil && *p.HasUserCorrected {
			if p.UserCorrectedPayee != nil {
				payee = *p.UserCorrectedPayee
			}
			if p.UserCorrectedCategory != nil {
				category = *p.UserCorrectedCategory
			}
			if p.UserCorrectedAccount != nil {
				account = *p.UserCorrectedAccount
			}
			source = "user_corrected"
		}

		if payee == "" || category == "" || account == "" {
			log.Warn("skipping prediction with empty labels", "id", p.ID)
			failed++
			continue
		}

		// Generate embedding from cleaned text
		embedding, err := ollamaClient.Embed(ctx, "bge-m3", cleanedEmailText)
		if err != nil {
			log.Error("failed to embed", "id", p.ID, "error", err)
			failed++
			continue
		}

		embeddingStr := db.VectorToString(embedding)

		var txnID *uuid.UUID = nil
		if p.TransactionID != "" {
			val, _ := uuid.Parse(p.TransactionID)
			txnID = &val
		}

		data := model.TransactionEmbedding{
			BudgetID:      budgetID,
			EmbeddingText: cleanedEmailText,
			Payee:         payee,
			Category:      category,
			Account:       account,
			Amount:        p.Amount,
			TransactionID: txnID,
			Source:        source,
		}
		slog.Debug("data", "data", data)

		if err := embeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
			log.Error("failed to upsert embedding", "id", p.ID, "error", err)
			failed++
			continue
		}

		success++
		if (i+1)%50 == 0 {
			log.Info("progress", "processed", i+1, "total", len(predictions), "success", success, "failed", failed)
		}

		// Small delay to not overwhelm Ollama
		time.Sleep(100 * time.Millisecond)
	}

	log.Info("backfill complete", "total", len(predictions), "success", success, "failed", failed)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
