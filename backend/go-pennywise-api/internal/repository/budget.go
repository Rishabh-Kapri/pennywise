package repository

import (
	"context"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetRepository interface {
	db.BaseRepositoryInterface
	GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error)
	GetById(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Budget, error)
	Create(ctx context.Context, tx pgx.Tx, name string, userID uuid.UUID) (*model.Budget, error)
	UpdateById(ctx context.Context, tx pgx.Tx, id uuid.UUID, budget model.Budget) error
	IsOwnedByUser(ctx context.Context, budgetID uuid.UUID, userID uuid.UUID) (bool, error)
}

type budgetRepo struct {
	db.BaseRepository
}

func NewBudgetRepository(pool *pgxpool.Pool) BudgetRepository {
	return &budgetRepo{BaseRepository: db.NewBaseRepository(pool)}
}

func (r *budgetRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error) {
	rows, err := r.Executor(nil).Query(
		ctx, `
			SELECT id, user_id, name, is_selected, created_at, updated_at, metadata
			FROM budgets
			WHERE user_id = $1 AND deleted = FALSE
		`, userID,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var budgets []model.Budget

	for rows.Next() {
		var b model.Budget
		err := rows.Scan(&b.ID, &b.UserID, &b.Name, &b.IsSelected, &b.CreatedAt, &b.UpdatedAt, &b.Metadata)
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
				SELECT id, user_id, name, is_selected, created_at, updated_at, metadata
				FROM budgets
				WHERE id = $1 AND deleted = FALSE
			`, id,
	).Scan(&budget.ID, &budget.UserID, &budget.Name, &budget.IsSelected, &budget.CreatedAt, &budget.UpdatedAt, &budget.Metadata)
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

func (r *budgetRepo) Create(ctx context.Context, tx pgx.Tx, name string, userID uuid.UUID) (*model.Budget, error) {
	var createdBudget model.Budget
	err := r.Executor(tx).QueryRow(
		ctx, `
			INSERT INTO budgets (name, user_id, is_selected, created_at, updated_at) 
			VALUES ($1, $2, FALSE, NOW(), NOW())
			RETURNING id, name, is_selected
			`, name, userID,
	).Scan(&createdBudget.ID, &createdBudget.Name, &createdBudget.IsSelected)
	if err != nil {
		return nil, err
	}
	createdBudget.UserID = userID
	return &createdBudget, nil
}

func (r *budgetRepo) UpdateById(ctx context.Context, tx pgx.Tx, id uuid.UUID, budget model.Budget) error {
	cmdTag, err := r.Executor(tx).Exec(
		ctx, `
			UPDATE budgets SET name = $1, is_selected = $2, metadata = $3, updated_at = NOW()
			WHERE id = $4 AND deleted = FALSE
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

func (r *budgetRepo) IsOwnedByUser(ctx context.Context, budgetID uuid.UUID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM budgets WHERE id = $1 AND user_id = $2 AND deleted = FALSE)`,
		budgetID, userID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
