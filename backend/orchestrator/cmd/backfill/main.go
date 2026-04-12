package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	db "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/repository"

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
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	cfg := config.Load()

	pennywiseAPI := os.Getenv("PENNYWISE_API")
	if pennywiseAPI == "" {
		log.Fatal("PENNYWISE_API environment variable is required")
	}

	budgetIDStr := os.Getenv("BUDGET_ID")
	if budgetIDStr == "" {
		log.Fatal("BUDGET_ID environment variable is required")
	}
	budgetID, err := uuid.Parse(budgetIDStr)
	if err != nil {
		log.Fatalf("Invalid BUDGET_ID: %v", err)
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

	ollamaClient := client.NewOllamaClient(cfg.OllamaURL)
	embeddingRepo := repository.NewTransactionEmbeddingRepository(dbConn)

	ctx := context.Background()

	// Fetch predictions from go-pennywise-api
	predictions, err := fetchPredictions(pennywiseAPI, budgetIDStr)
	if err != nil {
		log.Fatalf("Failed to fetch predictions: %v", err)
	}

	slog.Info("fetched predictions", "count", len(predictions))

	success, failed := 0, 0
	for i, p := range predictions {
		if p.EmailText == "" {
			slog.Warn("skipping prediction with empty email text", "id", p.ID)
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
		source := "prediction"

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
			slog.Warn("skipping prediction with empty labels", "id", p.ID)
			failed++
			continue
		}

		// Generate embedding from cleaned text
		embedding, err := ollamaClient.Embed(ctx, "bge-m3", cleanedEmailText)
		if err != nil {
			slog.Error("failed to embed", "id", p.ID, "error", err)
			failed++
			continue
		}

		embeddingStr := db.VectorToString(embedding)

		txnID, _ := uuid.Parse(p.TransactionID)
		data := model.TransactionEmbedding{
			BudgetID:      budgetID,
			EmbeddingText: cleanedEmailText,
			Payee:         payee,
			Category:      category,
			Account:       account,
			Amount:        p.Amount,
			TransactionID: &txnID,
			Source:        source,
		}
		slog.Debug("data", "data", data)

		if err := embeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
			slog.Error("failed to upsert embedding", "id", p.ID, "error", err)
			failed++
			continue
		}

		success++
		if (i+1)%50 == 0 {
			slog.Info("progress", "processed", i+1, "total", len(predictions), "success", success, "failed", failed)
		}

		// Small delay to not overwhelm Ollama
		time.Sleep(100 * time.Millisecond)
	}

	slog.Info("backfill complete", "total", len(predictions), "success", success, "failed", failed)
}

func fetchPredictions(apiURL string, budgetID string) ([]Prediction, error) {
	url := fmt.Sprintf("%s/api/predictions", apiURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authUserId", "fb7c7893-84f7-4344-a861-064985d442f7")
	req.Header.Set("X-Budget-ID", budgetID)
	// req.Header.Set("Authorization", "Bearer "+authToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var predictions []Prediction
	if err := json.Unmarshal(body, &predictions); err != nil {
		return nil, err
	}

	return predictions, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
