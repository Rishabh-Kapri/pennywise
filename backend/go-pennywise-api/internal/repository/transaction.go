package repository

import (
	"context"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Transaction, error)
	// GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error)
	// Create(ctx context.Context, txn model.Transaction) error
	// DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}

type transactionRepo struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) TransactionRepository {
	return &transactionRepo{db: db}
}

func (r *transactionRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Transaction, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, budget_id, date, payee_id, category_id, account_id, note, amount, transfer_account_id, transfer_transaction_id, created_at, updated_at
		 FROM transactions 
		 WHERE budget_id = $1 AND deleted = FALSE
		 ORDER BY date DESC, updated_at DESC;`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []model.Transaction
	for rows.Next() {
		var txn model.Transaction
		err := rows.Scan(&txn.ID, &txn.BudgetID, &txn.Date, &txn.PayeeID, &txn.CategoryID, &txn.AccountID, &txn.Note, &txn.Amount, &txn.TransferAccountID, &txn.TransferTransactionID, &txn.CreatedAt, &txn.UpdatedAt)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}
	return transactions, nil
}
