package repository

import (
	"context"
	"errors"
	"time"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryGroupRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.CategoryGroup, error)
	Create(ctx context.Context, categoryGroup model.CategoryGroup) error
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, categoryGroup model.CategoryGroup) error
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
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

func (r *categoryGroupRepo) Create(ctx context.Context, categoryGroup model.CategoryGroup) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO category_groups (id, name, budget_id, hidden, is_system, deleted, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		categoryGroup.ID,
		categoryGroup.Name,
		categoryGroup.BudgetID,
		categoryGroup.Hidden,
		categoryGroup.IsSystem,
		categoryGroup.Deleted,
		categoryGroup.CreatedAt,
		categoryGroup.UpdatedAt,
	)
	return err
}

func (r *categoryGroupRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, categoryGroup model.CategoryGroup) error {
	_, err := r.db.Exec(
		ctx,
		`UPDATE category_groups SET name = $1, hidden = $2, is_system = $3, updated_at = $4 WHERE id = $5 AND budget_id = $6`,
		categoryGroup.Name,
		categoryGroup.Hidden,
		categoryGroup.IsSystem,
		time.Now(),
		id,
		budgetId,
	)
	return err
}

func (r *categoryGroupRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	cmdTag, err := r.db.Exec(
		ctx,
		`UPDATE category_groups SET deleted = TRUE WHERE id = $1 AND budget_id = $2`,
		id,
		budgetId,
	)
	if cmdTag.RowsAffected() == 0 {
		return errors.New("Category group not found")
	}
	return err
}
