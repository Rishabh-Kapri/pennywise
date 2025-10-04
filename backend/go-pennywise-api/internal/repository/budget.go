package repository

import (
	"context"
	"fmt"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetRepository interface {
	BaseRepository
	// @TODO: add email in the future
	GetAll(ctx context.Context) ([]model.Budget, error)
	GetById(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Budget, error)
	Create(ctx context.Context, tx pgx.Tx, name string) (*model.Budget, error)
	UpdateById(ctx context.Context, tx pgx.Tx, id uuid.UUID, budget model.Budget) error
}

type budgetRepo struct {
	baseRepository
}

func NewBudgetRepository(db *pgxpool.Pool) BudgetRepository {
	return &budgetRepo{baseRepository: NewBaseRepository(db)}
}

func (r *budgetRepo) GetAll(ctx context.Context) ([]model.Budget, error) {
	rows, err := r.Executor(nil).Query(
		ctx, `
			SELECT id, name, is_selected, created_at, updated_at, metadata
			FROM budgets
		`,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var budgets []model.Budget

	for rows.Next() {
		var b model.Budget
		err := rows.Scan(&b.ID, &b.Name, &b.IsSelected, &b.CreatedAt, &b.UpdatedAt, &b.Metadata)
		if err != nil {
			return nil, err
		}
		budgets = append(budgets, b)
	}
	return budgets, nil
}

func (r *budgetRepo) GetById(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Budget, error) {
	var budget model.Budget
	err := r.Executor(tx).QueryRow(
		ctx, `
				SELECT id, name, is_selected, created_at, updated_at, metadata
				FROM budgets
				WHERE id = $1
			`, id,
	).Scan(&budget.ID, &budget.Name, &budget.IsSelected, &budget.CreatedAt, &budget.UpdatedAt, &budget.Metadata)
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

func (r *budgetRepo) Create(ctx context.Context, tx pgx.Tx, name string) (*model.Budget, error) {
	var createdBudget model.Budget
	err := r.Executor(tx).QueryRow(
		ctx, `
			INSERT INTO budgets (name, is_selected, created_at, updated_at) 
			VALUES ($1, FALSE, NOW(), NOW())
			RETURNING id, name, is_selected
			`, name,
	).Scan(&createdBudget.ID, &createdBudget.Name, &createdBudget.IsSelected)
	if err != nil {
		return nil, err
	}
	return &createdBudget, nil
}

func (r *budgetRepo) UpdateById(ctx context.Context, tx pgx.Tx, id uuid.UUID, budget model.Budget) error {
	cmdTag, err := r.Executor(tx).Exec(
		ctx, `
			UPDATE budgets SET name = $1, is_selected = $2, metadata = $3, updated_at = NOW()
			WHERE id = $4
			`, budget.Name, budget.IsSelected, budget.Metadata, id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("budget not found for id %v", id)
	}
	return nil
}
