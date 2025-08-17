package repository

import (
	"context"
	"errors"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Account, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Account, error)
	Create(ctx context.Context, account model.Account) (*model.Account, error)
}

type accountRepo struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) AccountRepository {
	return &accountRepo{db: db}
}

// createAccountWithPayee creates an account with a payee
// first creates an account,
// then creates a payee using the accountId as transferAccountId
// then updates the account with the payeeId as transferPayeeId
func createAccountWithPayee(ctx context.Context, db *pgxpool.Pool, account model.Account) (*model.Account, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	// create an account
	var createdAcc model.Account
	err = tx.QueryRow(
		ctx,
		`INSERT INTO accounts (
		  name, type, budget_id, closed, deleted, created_at, updated_at
		 ) VALUES ($1, $2, $3, FALSE, FALSE, NOW(), NOW()) 
		 RETURNING id, name, type, budget_id`,
		account.Name, account.Type, account.BudgetID,
	).Scan(&createdAcc.ID, &createdAcc.Name, &createdAcc.Type, &createdAcc.BudgetID)
	if err != nil {
		return nil, err
	}

	// create a payee with transfer account id
	var payeeId uuid.UUID
	payeeName := "Transfer : " + account.Name
	err = tx.QueryRow(
		ctx,
		`INSERT INTO payees (name, budget_id,  transfer_account_id, deleted, created_at, updated_at)
		   VALUES ($1, $2, $3, FALSE, NOW(), NOW()) 
		 RETURNING id`,
		payeeName, account.BudgetID, createdAcc.ID,
	).Scan(&payeeId)
	if err != nil {
		return nil, err
	}

	// udpate account with transfer payee id
	_, err = tx.Exec(
		ctx,
		`UPDATE accounts SET transfer_payee_id = $1 WHERE id = $2`,
		payeeId, createdAcc.ID,
	)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &createdAcc, nil
}

func (r *accountRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Account, error) {
	rows, err := r.db.Query(
		ctx, `
		SELECT 
		  accounts.id,
		  accounts.name,
		  accounts.budget_id,
		  accounts.transfer_payee_id,
		  accounts.type,
		  accounts.closed,
		  accounts.created_at,
		  accounts.updated_at,
		  COALESCE((
				SELECT SUM(transactions.amount)
				FROM transactions
				WHERE transactions.account_id = accounts.id AND transactions.deleted = FALSE
		  ), 0) as balance
		FROM accounts
		WHERE budget_id = $1 AND deleted = FALSE
		`, budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []model.Account
	for rows.Next() {
		var a model.Account
		err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.BudgetID,
			&a.TransferPayeeID,
			&a.Type,
			&a.Closed,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.Balance,
		)
		if err != nil {
			errorMsg := errors.New("Error while parsing account rows: ")
			return nil, errors.Join(errorMsg, err)
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (r *accountRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Account, error) {
	rows, err := r.db.Query(
		ctx, `
			SELECT 
				accounts.id,
				accounts.name,
				accounts.budget_id,
				accounts.transfer_payee_id,
				accounts.type,
				accounts.closed,
				accounts.created_at,
				accounts.updated_at,
				COALESCE((
					SELECT SUM(transactions.amount)
					FROM transactions
					WHERE transactions.account_id = accounts.id AND transactions.deleted = FALSE
				), 0) as balance
			FROM accounts
		  WHERE budget_id = $1 AND deleted = FALSE AND name LIKE $2
		`,
		budgetId, "%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	var accounts []model.Account
	defer rows.Close()
	for rows.Next() {
		var a model.Account
		err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.BudgetID,
			&a.TransferPayeeID,
			&a.Type,
			&a.Closed,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.Balance,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}

	return accounts, nil
}

func (r *accountRepo) Create(ctx context.Context, account model.Account) (*model.Account, error) {
	createdAcc, err := createAccountWithPayee(ctx, r.db, account)

	return createdAcc, err
}
