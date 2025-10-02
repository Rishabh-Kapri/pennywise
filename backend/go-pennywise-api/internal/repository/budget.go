package repository

import (
	"context"
	"fmt"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetRepository interface {
	// @TODO: add email in the future
	GetAll(ctx context.Context) ([]model.Budget, error)
	Create(ctx context.Context, name string) error
	UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error
}

type budgetRepo struct {
	db *pgxpool.Pool
}

func NewBudgetRepository(db *pgxpool.Pool) BudgetRepository {
	return &budgetRepo{db}
}

func (r *budgetRepo) GetAll(ctx context.Context) ([]model.Budget, error) {
	rows, err := r.db.Query(
		ctx, `
			SELECT id, name, is_selected, created_at, updated_at
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
		err := rows.Scan(&b.ID, &b.Name, &b.IsSelected, &b.CreatedAt, &b.UpdatedAt)
		if err != nil {
			return nil, err
		}
		budgets = append(budgets, b)
	}
	return budgets, nil
}

func (r *budgetRepo) Create(ctx context.Context, name string) error {
	_, err := r.db.Exec(
		ctx, `
		  INSERT INTO budgets (name, is_selected, created_at, updated_at) 
		  VALUES ($1, FALSE, NOW(), NOW())
		`, name,
	)
	return err
}

func (r *budgetRepo) UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error {
	cmdTag, err := r.db.Exec(
		ctx, `
		  UPDATE budgets SET name = $1, is_selected = $2, updated_at = NOW()
		  WHERE id = $3
		`, budget.Name, budget.IsSelected, id,
		)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("budget not found for id %v", id)
	}
	return nil
}
