package main

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BackfillDeps holds all dependencies needed by backfill operations.
type BackfillDeps struct {
	OllamaClient  *client.OllamaClient
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
	dbConn *pgxpool.Pool,
) (*BackfillDeps, *transport.Client) {
	ctx := context.Background()
	otelCfg := otelSDK.Load()
	tel, err := otelSDK.NewTelemetry(ctx, *otelCfg)
	if err != nil {
		logger.Fatal("error while initializing telemetry", err)
	}
	defer tel.Shutdown(ctx)
	// Ollama client via shared transport
	ollamaEngine := httpclient.NewHttpTransport(cfg.OllamaURL)
	ollamaTransport := transport.NewClient("ollama", ollamaEngine)
	ollamaClient := client.NewOllamaClient(ollamaTransport, tel.Tracer)

	// Pennywise API client via shared transport
	pennywiseEngine := httpclient.NewHttpTransport(pennywiseAPI)
	pennywiseClient := transport.NewClient("pennywise-api", pennywiseEngine)

	return &BackfillDeps{
		OllamaClient:  ollamaClient,
		EmbeddingRepo: repository.NewTransactionEmbeddingRepository(dbConn),
		PayeeRepo:     repository.NewPayeesRepository(dbConn),
		PayeeRuleRepo: repository.NewPayeeRuleRepository(dbConn),
		CategoryRepo:  repository.NewCategoryRepository(dbConn),
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
			return d.PayeeRepo.Create(ctx, nil, newPayee)
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
			return d.CategoryRepo.Create(ctx, nil, newCategory)
		}
		return nil, errs.New(errs.CodeInternalError, "category not found")
	}
	return &foundCategory[0], nil
}
