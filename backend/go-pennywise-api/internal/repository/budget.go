package repository

import (
	"context"

	"pennywise-api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetRepository interface {
	// @TODO: add email in the future
	GetAll(ctx context.Context) ([]model.Budget, error)
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
