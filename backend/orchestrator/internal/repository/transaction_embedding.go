package repository

import (
	"context"
	"fmt"
	"pennywise-shared/db"

	"orchestrator/internal/model"

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

	if data.TransactionID != nil {
		_, err := executor.Exec(
			ctx, `
				INSERT INTO transaction_embeddings (
					budget_id, embedding_text, embedding, payee, category, account,
					amount, transaction_id, source, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
				ON CONFLICT (transaction_id) WHERE transaction_id IS NOT NULL
				DO UPDATE SET
					embedding_text = EXCLUDED.embedding_text,
					embedding = EXCLUDED.embedding,
					payee = EXCLUDED.payee,
					category = EXCLUDED.category,
					account = EXCLUDED.account,
					amount = EXCLUDED.amount,
					source = EXCLUDED.source,
					updated_at = NOW()
			`,
			data.BudgetID, data.EmbeddingText, embeddingStr,
			data.Payee, data.Category, data.Account,
			data.Amount, data.TransactionID, data.Source,
		)
		return err
	}

	_, err := executor.Exec(
		ctx, `
			INSERT INTO transaction_embeddings (
				budget_id, embedding_text, embedding, payee, category, account,
				amount, source, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		`,
		data.BudgetID, data.EmbeddingText, embeddingStr,
		data.Payee, data.Category, data.Account,
		data.Amount, data.Source,
	)
	return err
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
