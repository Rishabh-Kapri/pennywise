package repository

import (
	"context"
	"errors"
	"log"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PayeesRepository interface {
	BaseRepository
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Payee, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Payee, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Payee, error)
	GetByIdTx(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) (*model.Payee, error)
	Create(ctx context.Context, tx pgx.Tx, payee model.Payee) (*model.Payee, error)
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payee model.Payee) error
}

type payeeRepo struct {
	baseRepository
}

func NewPayeesRepository(db *pgxpool.Pool) PayeesRepository {
	return &payeeRepo{baseRepository: NewBaseRepository(db)}
}

func (r *payeeRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Payee, error) {
	rows, err := r.Executor(nil).Query(
		ctx, `
		SELECT id, name, budget_id, transfer_account_id, created_at, updated_at
		FROM payees WHERE budget_id = $1 AND deleted = FALSE`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payees []model.Payee
	for rows.Next() {
		var payee model.Payee
		err := rows.Scan(&payee.ID, &payee.Name, &payee.BudgetID, &payee.TransferAccountID, &payee.CreatedAt, &payee.UpdatedAt)
		if err != nil {
			return nil, err
		}
		payees = append(payees, payee)
	}
	return payees, nil
}

func (r *payeeRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Payee, error) {
	log.Printf("%v %v", budgetId, query)
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT id, name, budget_id, transfer_account_id, created_at, updated_at FROM payees 
		   WHERE budget_id = $1 AND deleted = FALSE AND name = $2`,
		// budgetId, "%"+query+"%",
		budgetId, query,
		// budgetId, "[[:<:]]"+query+"[[:>:]]",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var payees []model.Payee
	for rows.Next() {
		var payee model.Payee
		err := rows.Scan(&payee.ID, &payee.Name, &payee.BudgetID, &payee.TransferAccountID, &payee.CreatedAt, &payee.UpdatedAt)
		if err != nil {
			return nil, err
		}
		payees = append(payees, payee)
	}
	return payees, nil
}

func (r *payeeRepo) GetById(ctx context.Context, budgetId, id uuid.UUID) (*model.Payee, error) {
	var payee model.Payee
	err := r.Executor(nil).QueryRow(
		ctx, `
		  SELECT id, name, budget_id, transfer_account_id
		  FROM payees
		  WHERE id = $1 AND budget_id = $2 AND deleted = FALSE
		`, id, budgetId,
	).Scan(
		&payee.ID,
		&payee.Name,
		&payee.BudgetID,
		&payee.TransferAccountID,
	)
	if err != nil {
		return nil, err
	}
	return &payee, nil
}

func (r *payeeRepo) GetByIdTx(ctx context.Context, tx pgx.Tx, budgetId, id uuid.UUID) (*model.Payee, error) {
	var payee model.Payee
	err := tx.QueryRow(
		ctx, `
		  SELECT id, name, budget_id, transfer_account_id
		  FROM payees
		  WHERE id = $1 AND budget_id = $2 AND deleted = FALSE
		`, id, budgetId,
	).Scan(
		&payee.ID,
		&payee.Name,
		&payee.BudgetID,
		&payee.TransferAccountID,
	)
	if err != nil {
		return nil, err
	}
	return &payee, nil
}

func (r *payeeRepo) Create(ctx context.Context, tx pgx.Tx, payee model.Payee) (*model.Payee, error) {
	var createdPayee model.Payee

	err := r.Executor(tx).QueryRow(
		ctx, `
				INSERT INTO payees (
				name, budget_id, transfer_account_id, deleted, created_at, updated_at
				) VALUES ($1, $2, $3, FALSE, NOW(), NOW())
				RETURNING id, name, transfer_account_id
			`, payee.Name,
		payee.BudgetID,
		payee.TransferAccountID,
	).Scan(&createdPayee.ID, &createdPayee.Name, &createdPayee.TransferAccountID)
	if err != nil {
		return nil, err
	}
	return &createdPayee, nil
}

func (r *payeeRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	cmdTag, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE payees SET
		   deleted = TRUE,
		   updated_at = NOW()
		WHERE id = $1 AND budget_id = $2`,
		id, budgetId,
	)

	if cmdTag.RowsAffected() == 0 {
		return errors.New("Payee not found")
	}

	return err
}

func (r *payeeRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payee model.Payee) error {
	cmdTag, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE payees SET
		   name = $1,
			 updated_at = NOW()
		WHERE id = $2 AND budget_id = $3`,
		payee.Name, id, budgetId,
	)

	if cmdTag.RowsAffected() == 0 {
		return errors.New("Payee not found")
	}

	return err
}
