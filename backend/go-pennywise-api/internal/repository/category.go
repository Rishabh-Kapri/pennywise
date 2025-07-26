package repository

import (
	"context"
	"time"

	"pennywise-api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository interface {
	GetAll(ctx context.Context, budgetId string) ([]model.Category, error)
	// GetById(ctx context.Context, id string) (model.Category, error)
	Create(ctx context.Context, category model.Category) error
}

type categoryRepo struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) CategoryRepository {
	return &categoryRepo{db: db}
}

func (r *categoryRepo) GetAll(ctx context.Context, budgetId string) ([]model.Category, error) {
	rows, err := r.db.Query(
		ctx,
		"SELECT id, name, category_group_id, hidden, note, created_at, updated_at FROM categories WHERE budget_id = $1 AND deleted = FALSE",
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var c model.Category
		err := rows.Scan(&c.ID, &c.Name, &c.CategoryGroupID, &c.Hidden, &c.Note, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (r *categoryRepo) Create(ctx context.Context, category model.Category) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO categories (
			budget_id, name, category_group_id, note, hidden, is_system, deleted, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		category.BudgetID, category.Name, category.CategoryGroupID, category.Note, false, false, false, time.Now(), time.Now(),
	)
	return err
}
