package main

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"

	"github.com/google/uuid"
)

// BackfillDeps holds all dependencies needed by backfill operations.
type BackfillDeps struct {
	Ollama        *ollamaClient
	EmbeddingRepo repository.TransactionEmbeddingRepository
	PayeeRepo     repository.PayeesRepository
	PayeeRuleRepo repository.PayeeRuleRepository
	CategoryRepo  repository.CategoryRepository
	BudgetID      uuid.UUID
}

// initDeps creates all clients and repositories needed for backfilling.
// Returns the deps, the pennywise API transport client (for loading predictions), and a cleanup function.
func initDeps(
	cfg config.Config,
	pennywiseAPI string,
	budgetID uuid.UUID,
	database *factory.Database,
) (*BackfillDeps, *transport.Client) {
	// Ollama client via shared transport
	ollama := &ollamaClient{
		client:      transport.NewClient("ollama", httpclient.NewHttpTransport(cfg.OllamaURL)),
		callTimeout: cfg.LLMCallTimeout,
	}

	// Pennywise API client via shared transport
	pennywiseEngine := httpclient.NewHttpTransport(pennywiseAPI)
	pennywiseClient := transport.NewClient("pennywise-api", pennywiseEngine)

	return &BackfillDeps{
		Ollama:        ollama,
		EmbeddingRepo: database.TransactionEmbedding,
		PayeeRepo:     database.Payee,
		PayeeRuleRepo: database.PayeeRule,
		CategoryRepo:  database.Category,
		BudgetID:      budgetID,
	}, pennywiseClient
}

func (d *BackfillDeps) getPayee(
	ctx context.Context,
	budgetID uuid.UUID,
	payeeName string,
	shouldCreate bool,
) (*model.Payee, error) {
	foundPayee, err := d.PayeeRepo.Search(ctx, budgetID, payeeName)
	if err != nil {
		return nil, err
	}
	if len(foundPayee) == 0 {
		if shouldCreate {
			newPayee := model.Payee{
				Name:     payeeName,
				BudgetID: budgetID,
			}
			return d.PayeeRepo.Create(ctx, newPayee)
		}
		return nil, errs.New(errs.CodeInternalError, "payee not found")
	}
	return &foundPayee[0], nil
}

func (d *BackfillDeps) getCategory(
	ctx context.Context,
	budgetID uuid.UUID,
	categoryName string,
	shouldCreate bool,
) (*model.Category, error) {
	foundCategory, err := d.CategoryRepo.Search(ctx, budgetID, categoryName)
	if err != nil {
		return nil, err
	}
	if len(foundCategory) == 0 {
		if shouldCreate {
			newCategory := model.Category{
				Name:            categoryName,
				BudgetID:        budgetID,
				CategoryGroupID: uuid.New(),
			}
			return d.CategoryRepo.Create(ctx, newCategory)
		}
		return nil, errs.New(errs.CodeInternalError, "category not found")
	}
	return &foundCategory[0], nil
}
