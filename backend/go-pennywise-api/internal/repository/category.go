package repository

import (
	"context"
	"errors"
	"log"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Category, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Category, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error)
	Create(ctx context.Context, category model.Category) error
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, category model.Category) error
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

func (r *categoryRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Category, error) {
	log.Printf("%v %v", budgetId, query)
	rows, err := r.db.Query(
		ctx,
		`SELECT id, name, budget_id, category_group_id, hidden, note, is_system, created_at, updated_at FROM categories 
		   WHERE budget_id  = $1 AND deleted = FALSE AND name LIKE $2`,
		budgetId, "%"+query+"%",
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
		) VALUES ($1, $2, $3, $4, FALSE, FALSE, NOW(), NOW())`,
		category.BudgetID, category.Name, category.CategoryGroupID, category.Note,
	)
	return err
}

func (r *categoryRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	cmdTag, err := r.db.Exec(
		ctx,
		`UPDATE categories SET 
	    deleted = TRUE
		  updated_at = NOW()
		WHERE id = $1 AND budget_id = $2`,
		id, budgetId,
	)

	if cmdTag.RowsAffected() == 0 {
		return errors.New("Category not found")
	}

	return err
}

func (r *categoryRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, category model.Category) error {
	_, err := r.db.Exec(
		ctx,
		`UPDATE categories SET 
		    name = $1,
				category_group_id = $2,
			  note = $3,
			  hidden = $4,
				is_system = $5,
				updated_at = NOW()
		WHERE budget_id = $6 AND id = $7`,
		category.Name,
		category.CategoryGroupID,
		category.Note,
		category.Hidden,
		category.IsSystem,
		budgetId,
		id,
	)

	return err
}
