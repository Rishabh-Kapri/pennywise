package repository

import (
	"context"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryGroupRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.CategoryGroup, error)
	// GetById(ctx context.Context, id string) (model.CategoryGroup, error)
	// Create(ctx context.Context, category model.CategoryGroup) error
}

type categoryGroupRepo struct {
	db *pgxpool.Pool
}

func NewCategoryGroupRepository(db *pgxpool.Pool) CategoryGroupRepository {
	return &categoryGroupRepo{db: db}
}

func (r *categoryGroupRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.CategoryGroup, error) {
	rows, err := r.db.Query(
		ctx,
		"SELECT id, name, budget_id, hidden, is_system, created_at, updated_at FROM category_groups WHERE budget_id = $1 AND deleted = $2",
		budgetId, false,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []model.CategoryGroup
	for rows.Next() {
		var g model.CategoryGroup
		err := rows.Scan(&g.ID, &g.Name, &g.BudgetID, &g.Hidden, &g.IsSystem, &g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// func (r)
