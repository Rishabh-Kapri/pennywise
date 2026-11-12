package postgres

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type accountRepo struct {
	BaseRepository
}

func NewAccountRepository(pool *pgxpool.Pool) repository.AccountRepository {
	return &accountRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *accountRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Account, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
		SELECT 
		  accounts.id,
		  accounts.name,
		  accounts.budget_id,
		  accounts.transfer_payee_id,
		  accounts.type,
		  COALESCE(accounts.bank, ''),
		  COALESCE(accounts.suffix, ''),
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

	accounts := make([]model.Account, 0)
	for rows.Next() {
		var a model.Account
		err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.BudgetID,
			&a.TransferPayeeID,
			&a.Type,
			&a.Bank,
			&a.Suffix,
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

func (r *accountRepo) GetById(ctx context.Context, budgetId uuid.UUID, accountId uuid.UUID) (*model.Account, error) {
	var a model.Account
	err := r.Executor(ctx).QueryRow(
		ctx, `
		  SELECT id, name, budget_id, transfer_payee_id, type, COALESCE(bank, ''), COALESCE(suffix, ''), closed, created_at, updated_at 
		  FROM accounts 
		  WHERE id = $1 AND budget_id = $2 AND deleted = FALSE
		`,
		accountId, budgetId,
	).Scan(
		&a.ID,
		&a.Name,
		&a.BudgetID,
		&a.TransferPayeeID,
		&a.Type,
		&a.Bank,
		&a.Suffix,
		&a.Closed,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *accountRepo) GetBySuffix(ctx context.Context, budgetId uuid.UUID, suffix string) (*model.Account, error) {
	logger.Logger(ctx).Info("getting account by suffix", "budgetId", budgetId, "suffix", suffix)
	var a model.Account
	err := r.Executor(ctx).QueryRow(
		ctx, `
		  SELECT id, name, budget_id, transfer_payee_id, type, COALESCE(bank, ''), COALESCE(suffix, ''), closed, created_at, updated_at 
		  FROM accounts 
		  WHERE budget_id = $1 AND deleted = FALSE AND suffix = $2
		`,
		budgetId, suffix,
	).Scan(
		&a.ID,
		&a.Name,
		&a.BudgetID,
		&a.TransferPayeeID,
		&a.Type,
		&a.Bank,
		&a.Suffix,
		&a.Closed,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *accountRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Account, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
			SELECT 
				accounts.id,
				accounts.name,
				accounts.budget_id,
				accounts.transfer_payee_id,
				accounts.type,
				COALESCE(accounts.bank, ''),
				COALESCE(accounts.suffix, ''),
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
	accounts := make([]model.Account, 0)
	defer rows.Close()
	for rows.Next() {
		var a model.Account
		err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.BudgetID,
			&a.TransferPayeeID,
			&a.Type,
			&a.Bank,
			&a.Suffix,
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
	var createdAcc model.Account
	err := r.Executor(ctx).QueryRow(
		ctx,
		`INSERT INTO accounts (
		  name, type, bank, suffix, budget_id, closed, deleted, created_at, updated_at
		 ) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, FALSE, FALSE, NOW(), NOW()) 
		 RETURNING id, name, type, COALESCE(bank, ''), COALESCE(suffix, ''), budget_id`,
		account.Name, account.Type, account.Bank, account.Suffix, account.BudgetID,
	).Scan(&createdAcc.ID, &createdAcc.Name, &createdAcc.Type, &createdAcc.Bank, &createdAcc.Suffix, &createdAcc.BudgetID)
	if err != nil {
		return nil, err
	}
	return &createdAcc, nil
}

func (r *accountRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, account model.Account) (*model.Account, error) {
	var a model.Account
	err := r.Executor(ctx).QueryRow(
		ctx,
		`UPDATE accounts SET
		   name = $1,
		   bank = NULLIF($2, ''),
		   suffix = NULLIF($3, ''),
		   closed = $4,
		   updated_at = NOW()
		 WHERE id = $5 AND budget_id = $6 AND deleted = FALSE
		 RETURNING id, name, budget_id, transfer_payee_id, type, COALESCE(bank, ''), COALESCE(suffix, ''), closed, created_at, updated_at`,
		account.Name, account.Bank, account.Suffix, account.Closed, id, budgetId,
	).Scan(&a.ID, &a.Name, &a.BudgetID, &a.TransferPayeeID, &a.Type, &a.Bank, &a.Suffix, &a.Closed, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *accountRepo) UpdateTransferPayee(ctx context.Context, accountId uuid.UUID, payeeId uuid.UUID) error {
	_, err := r.Executor(ctx).Exec(
		ctx,
		`UPDATE accounts SET transfer_payee_id = $1 WHERE id = $2`,
		payeeId, accountId,
	)
	return err
}
