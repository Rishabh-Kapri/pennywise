package db

import (
	"context"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionEmbeddingRepository interface {
	SearchSimilar(ctx context.Context, budgetID uuid.UUID, amount float64, embeddingStr string, limit int) ([]model.TransactionEmbedding, error)
	Upsert(ctx context.Context, tx pgx.Tx, data model.TransactionEmbedding, embeddingStr string) error
}

type transactionEmbeddingRepository struct {
	BaseRepository
}

func NewTransactionEmbeddingRepository(pool *pgxpool.Pool) TransactionEmbeddingRepository {
	return &transactionEmbeddingRepository{BaseRepository: NewBaseRepository(pool)}
}

func (r *transactionEmbeddingRepository) SearchSimilar(ctx context.Context, budgetID uuid.UUID, amount float64, embeddingStr string, limit int) ([]model.TransactionEmbedding, error) {
	rows, err := r.Executor(nil).Query(
		ctx, `
			SELECT
				id,
				budget_id,
				embedding_text,
				payee_id,
				category_id,
				amount,
				source,
		    (embedding <=> $1) AS vector_distance,
		    -- Added NULLIF to handle zero amounts
        ABS(ABS(amount) - ABS($4)) / NULLIF(GREATEST(ABS(amount), ABS($4)), 0) AS amount_penalty,
				created_at,
				updated_at
			FROM transaction_embeddings
			WHERE budget_id = $2
			ORDER BY 
        (embedding <=> $1) +
        (COALESCE(ABS(ABS(amount) - ABS($4)) / NULLIF(GREATEST(ABS(amount), ABS($4)), 0), 0) * 0.15) ASC
			LIMIT $3
		`, embeddingStr, budgetID, limit, amount,
	)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "SearchSimilar", err)
	}
	defer rows.Close()

	var results []model.TransactionEmbedding
	for rows.Next() {
		var e model.TransactionEmbedding
		if err := rows.Scan(
			&e.ID,
			&e.BudgetID,
			&e.EmbeddingText,
			&e.PayeeID,
			&e.CategoryID,
			&e.Amount,
			&e.Source,
			&e.VectorDistance,
			&e.AmountPenalty,
			&e.CreatedAt,
			&e.UpdatedAt,
		); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, "SearchSimilar scan", err)
		}
		results = append(results, e)
	}
	return results, nil
}

func (r *transactionEmbeddingRepository) Upsert(ctx context.Context, tx pgx.Tx, data model.TransactionEmbedding, embeddingStr string) error {
	executor := r.Executor(tx)
	query := `
	  INSERT INTO transaction_embeddings (
	    budget_id, embedding_text, embedding, payee_id, category_id, amount, source
	  ) VALUES ($1, $2, $3, $4, $5, $6, $7)
	  ON CONFLICT (budget_id, embedding_text) 
	  DO UPDATE SET
      -- Keep a rolling average of the typical spend amount
	    AMOUNT = (EXCLUDED.AMOUNT + transaction_embeddings.amount) / 2,
	    -- if the user changed the payee and category, update it
	    category_id = EXCLUDED.category_id,
	    payee_id = EXCLUDED.payee_id,
	    updated_at = NOW()
	`
	_, err := executor.Exec(
		ctx,
		query,
		data.BudgetID, data.EmbeddingText, embeddingStr,
		data.PayeeID, data.CategoryID,
		data.Amount, data.Source,
	)
	return err
}
