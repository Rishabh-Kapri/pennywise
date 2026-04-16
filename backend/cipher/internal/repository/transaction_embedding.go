package repository

import (
	"context"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionEmbeddingRepository interface {
	SearchSimilar(ctx context.Context, budgetID uuid.UUID, embeddingStr string, limit int) ([]model.TransactionEmbedding, error)
	Upsert(ctx context.Context, tx pgx.Tx, data model.TransactionEmbedding, embeddingStr string) error
	DeleteByTransactionID(ctx context.Context, tx pgx.Tx, budgetID uuid.UUID, txnID uuid.UUID) error
}

type transactionEmbeddingRepository struct {
	db.BaseRepository
}

func NewTransactionEmbeddingRepository(pool *pgxpool.Pool) TransactionEmbeddingRepository {
	return &transactionEmbeddingRepository{BaseRepository: db.NewBaseRepository(pool)}
}

func (r *transactionEmbeddingRepository) SearchSimilar(ctx context.Context, budgetID uuid.UUID, embeddingStr string, limit int) ([]model.TransactionEmbedding, error) {
	rows, err := r.Executor(nil).Query(
		ctx, `
			SELECT
				id,
				budget_id,
				embedding_text,
				payee,
				category,
				account,
				amount,
				transaction_id,
				source,
				1 - (embedding <=> $2) AS similarity,
				created_at,
				updated_at
			FROM transaction_embeddings
			WHERE budget_id = $1
			ORDER BY embedding <=> $2 ASC
			LIMIT $3
		`, budgetID, embeddingStr, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("SearchSimilar: %w", err)
	}
	defer rows.Close()

	var results []model.TransactionEmbedding
	for rows.Next() {
		var e model.TransactionEmbedding
		if err := rows.Scan(
			&e.ID,
			&e.BudgetID,
			&e.EmbeddingText,
			&e.Payee,
			&e.Category,
			&e.Account,
			&e.Amount,
			&e.TransactionID,
			&e.Source,
			&e.Similarity,
			&e.CreatedAt,
			&e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("SearchSimilar scan: %w", err)
		}
		results = append(results, e)
	}
	return results, nil
}

func (r *transactionEmbeddingRepository) Upsert(ctx context.Context, tx pgx.Tx, data model.TransactionEmbedding, embeddingStr string) error {
	executor := r.Executor(tx)
	query := `
		WITH target AS (
				-- 1. Identify if a matching row exists by ID or by Text
				SELECT id FROM transaction_embeddings 
				WHERE (transaction_id = $8 AND transaction_id IS NOT NULL)
					 OR (embedding_text = $2 AND budget_id = $1)
				LIMIT 1
		),
		upsert AS (
				-- 2. Update the existing row if it was found
				UPDATE transaction_embeddings SET
						embedding_text = $2,
						embedding = $3,
						payee = $4,
						category = $5,
						account = $6,
						amount = $7,
						-- Link the transaction_id if the existing record was just text-based
						transaction_id = COALESCE(transaction_id, $8),
						source = $9,
						updated_at = NOW()
				FROM target
				WHERE transaction_embeddings.id = target.id
				RETURNING transaction_embeddings.id
		)
		-- 3. If nothing was updated, insert the new record
		INSERT INTO transaction_embeddings (
				budget_id, embedding_text, embedding, payee, category, account,
				amount, transaction_id, source, created_at, updated_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW()
		WHERE NOT EXISTS (SELECT 1 FROM target);
	`

	_, err := executor.Exec(
		ctx,
		query,
		data.BudgetID, data.EmbeddingText, embeddingStr,
		data.Payee, data.Category, data.Account,
		data.Amount, data.TransactionID, data.Source,
	)
	return err

	// _, err := executor.Exec(
	// 	ctx, `
	// 		INSERT INTO transaction_embeddings (
	// 			budget_id, embedding_text, embedding, payee, category, account,
	// 			amount, source, created_at, updated_at
	// 		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	// 	`,
	// 	data.BudgetID, data.EmbeddingText, embeddingStr,
	// 	data.Payee, data.Category, data.Account,
	// 	data.Amount, data.Source,
	// )
	// return err
}

func (r *transactionEmbeddingRepository) DeleteByTransactionID(ctx context.Context, tx pgx.Tx, budgetID uuid.UUID, txnID uuid.UUID) error {
	executor := r.Executor(tx)
	_, err := executor.Exec(
		ctx,
		`DELETE FROM transaction_embeddings WHERE budget_id = $1 AND transaction_id = $2`,
		budgetID, txnID,
	)
	return err
}
