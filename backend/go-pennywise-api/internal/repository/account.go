package repository

import (
	"context"
	"errors"

	"pennywise-api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountRepository interface {
	GetAll(ctx context.Context, budgetId string) ([]model.Account, error)
	Create(ctx context.Context, account model.Account) error
}

type accountRepo struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) AccountRepository {
	return &accountRepo{db: db}
}

func (r *accountRepo) GetAll(ctx context.Context, budgetId string) ([]model.Account, error) {
	rows, err := r.db.Query(
		ctx,
		"SELECT id, name, transfer_payee_id, type, closed, created_at, updated_at FROM accounts WHERE budget_id = $1 AND deleted = $2",
		budgetId, false,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []model.Account
	for rows.Next() {
		var a model.Account
		err := rows.Scan(&a.ID, &a.Name, &a.TransferPayeeID, &a.Type, &a.Closed, &a.CreatedAt, &a.UpdatedAt)
		if err != nil {
			errorMsg := errors.New("Error while parsing account rows: ")
			return nil, errors.Join(errorMsg, err)
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (r *accountRepo) Create(ctx context.Context, account model.Account) error {
	// @TODO: handle creation of account by creating transfer payee first, use db transactions
	_, err := r.db.Exec(
		ctx,
		"INSERT INTO accounts (budget_id, name, transfer_payee_id, type, closed, deleted, created_at) VALUES ($1, $2, $3, $4, false, false, NOW())",
		account.BudgetID, account.Name, account.TransferPayeeID, account.Type,
	)
	return err
}
