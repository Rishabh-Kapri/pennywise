package repository

import (
	"context"
	"log"
	"time"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Category, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error)
	Create(ctx context.Context, category model.Category) error
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}

type categoryRepo struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) CategoryRepository {
	return &categoryRepo{db: db}
}

func (r *categoryRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Category, error) {
	rows, err := r.db.Query(
		ctx,
		"SELECT id, name, budget_id, category_group_id, hidden, note, is_system, created_at, updated_at FROM categories WHERE budget_id = $1 AND deleted = FALSE",
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var c model.Category
		err := rows.Scan(&c.ID, &c.Name, &c.BudgetID, &c.CategoryGroupID, &c.Hidden, &c.Note, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (r *categoryRepo) GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error) {
	row := r.db.QueryRow(
		ctx,
		"SELECT id, name, budget_id, category_group_id, hidden, note, is_system, created_at, updated_at FROM categories WHERE budget_id = $1 AND deleted = FALSE AND id = $2",
		budgetId, id,
	)

	var c model.Category
	err := row.Scan(&c.ID, &c.Name, &c.BudgetID, &c.CategoryGroupID, &c.Hidden, &c.Note, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &c, nil
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

// @TODO: this is returning error even for valid idea
func (r *categoryRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	row := r.db.QueryRow(
		ctx,
		"DELETE FROM categories WHERE budget_id = $1 AND deleted = FALSE AND id = $2",
		budgetId, id,
	)
	var c any
	err := row.Scan(&c)
	log.Printf("deleted: %v", c)
	log.Printf("error while deleting id: %v", err)
	if err != nil {
		return err
	}

	return err
}
