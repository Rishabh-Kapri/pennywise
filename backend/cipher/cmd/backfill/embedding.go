package main

import (
	"context"
	"time"

	db "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

const embeddingModel = "bge-m3"

// backfillEmbedding generates a vector embedding for a single prediction
// and upserts it into the transaction_embeddings table (Phase 3 memory from cipher.md).
func (d *BackfillDeps) backfillEmbedding(ctx context.Context, p resolvedPrediction, parsed *parsedEmailText) error {
	log := logger.Logger(ctx)
	budgetID := d.BudgetID

	// Resolve payee → UUID
	foundPayee, err := d.getPayee(ctx, budgetID, p.Payee, false)
	if err != nil {
		return err
	}

	// Resolve category → UUID
	foundCategory, err := d.getCategory(ctx, budgetID, p.Category, false)
	if err != nil {
		return err
	}

	// For embedding, we only use transaction type (debit/credit) + merchant name (no upi handles)
	embeddingText := parsed.TransactionType + " " + parsed.MerchantName
	log.Info(
		"embedding",
		"id",
		p.ID,
		"text",
		embeddingText,
		"payee",
		foundPayee.Name,
		"category",
		foundCategory.Name,
	)

	// Generate bge-m3 embedding via Ollama
	embedding, err := d.OllamaClient.Embed(ctx, embeddingModel, embeddingText)
	if err != nil {
		return err
	}

	embeddingStr := db.VectorToString(embedding)

	data := model.TransactionEmbedding{
		BudgetID:      budgetID,
		EmbeddingText: embeddingText,
		PayeeID:       foundPayee.ID,
		CategoryID:    foundCategory.ID,
		Amount:        p.Amount,
		Source:        p.Source,
	}

	if err := d.EmbeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
		return err
	}

	// Small delay to avoid overwhelming Ollama
	time.Sleep(100 * time.Millisecond)
	return nil
}
