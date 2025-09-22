package repository

import (
	"context"
	"errors"
	"fmt"
	"log"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Category, error)
	GetInflowBalance(ctx context.Context, budgetId uuid.UUID) (float64, error)
	GetByFilter(ctx context.Context, budgetId uuid.UUID, filter model.CategoryFilter) ([]model.Category, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Category, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error)
	GetByIdSimplified(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error)
	GetByIdSimplifiedTx(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error)
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
		ctx, `
			SELECT 
				categories.id, 
		    categories.name, 
				categories.budget_id, 
				categories.category_group_id, 
				categories.hidden, 
				categories.note, 
				categories.is_system, 
				categories.created_at, 
				categories.updated_at,
		    COALESCE(
		      json_object_agg(monthly_budgets.month, monthly_budgets.budgeted) 
		      FILTER (WHERE monthly_budgets.month IS NOT NULL), '{}'
		    ) AS budgeted,
		    COALESCE(
		      (
		        SELECT json_object_agg(tx.month, tx.sum)
		        FROM (
		          SELECT
		            TO_CHAR(date_trunc('month', transactions.date::date), 'YYYY-MM') AS month,
		            SUM(transactions.amount) AS sum
		          FROM transactions
		          WHERE transactions.category_id = categories.id AND transactions.deleted = FALSE
		          GROUP BY month
		        ) AS tx
		      ), '{}'
		    ) AS activity,
		    COALESCE(
		      json_object_agg(monthly_budgets.month, monthly_budgets.carryover_balance) 
		      FILTER (WHERE monthly_budgets.month IS NOT NULL), '{}'
		    ) AS balance
			FROM categories 
		  LEFT JOIN monthly_budgets ON categories.id = monthly_budgets.category_id
			WHERE categories.budget_id = $1 AND categories.deleted = FALSE
		  GROUP BY categories.id
		`, budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var c model.Category
		err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.BudgetID,
			&c.CategoryGroupID,
			&c.Hidden,
			&c.Note,
			&c.IsSystem,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.Budgeted,
			&c.Activity,
			&c.Balance,
		)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (r *categoryRepo) GetInflowBalance(ctx context.Context, budgetId uuid.UUID) (float64, error) {
	var balance float64

	err := r.db.QueryRow(ctx, `
			WITH inflow_cat AS (
				SELECT id FROM categories
				WHERE budget_id = $1 AND is_system = TRUE AND deleted = FALSE
				LIMIT 1
			),
			total_txn AS (
				SELECT COALESCE(SUM(amount), 0) AS transaction_amount
				FROM transactions
				WHERE category_id = (SELECT id FROM inflow_cat) AND deleted = FALSE
			),
			total_budgeted AS (
				SELECT COALESCE(SUM(budgeted), 0) AS total_budgeted
				FROM monthly_budgets
			)
			SELECT (total_txn.transaction_amount - total_budgeted.total_budgeted)
			FROM total_txn, total_budgeted
		`, budgetId).Scan(&balance)
	if err != nil {
		return 0, err
	}
	return balance, err
}

func (r *categoryRepo) GetByFilter(ctx context.Context, budgetId uuid.UUID, filter model.CategoryFilter) ([]model.Category, error) {
	sql := `SELECT id, name, budget_id, category_group_id, hidden, note, is_system, created_at, updated_at FROM categories WHERE deleted = FALSE AND budget_id = $1`
	args := []any{budgetId}
	argIndex := 2 // $1 is budget_id
	if filter.IsSystem != nil {
		sql += fmt.Sprintf(" AND is_system = $%d", argIndex)
		args = append(args, *filter.IsSystem)
		argIndex++
	}
	log.Printf("%v", sql)
	log.Printf("%v", args)

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var c model.Category
		err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.BudgetID,
			&c.CategoryGroupID,
			&c.Hidden,
			&c.Note,
			&c.IsSystem,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
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
		ctx, `
			SELECT 
				categories.id,
				categories.name,
				categories.budget_id,
				categories.category_group_id,
				categories.hidden,
				categories.note,
				categories.is_system,
				categories.created_at,
				categories.updated_at,
				COALESCE(
					json_object_agg(monthly_budgets.month, monthly_budgets.budgeted)
					FILTER (WHERE monthly_budgets.month IS NOT NULL), '{}'
				) AS budgeted,
				COALESCE(
					(
						SELECT json_object_agg(tx.month, tx.sum)
						FROM (
							SELECT
								TO_CHAR(date_trunc('month', transactions.date::date), 'YYYY-MM') AS month,
								SUM(transactions.amount) AS sum
							FROM transactions
							WHERE transactions.category_id = categories.id AND transactions.deleted = FALSE
							GROUP BY month
						) AS tx
					), '{}'
				) AS activity,
				COALESCE(
					json_object_agg(monthly_budgets.month, monthly_budgets.carryover_balance) 
					FILTER (WHERE monthly_budgets.month IS NOT NULL), '{}'
				) AS balance
			FROM categories 
		  LEFT JOIN monthly_budgets ON categories.id = monthly_budgets.category_id
			WHERE categories.budget_id  = $1 AND categories.deleted = FALSE AND categories.name LIKE $2
		  GROUP BY categories.id
		`, budgetId, "%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var c model.Category
		err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.BudgetID,
			&c.CategoryGroupID,
			&c.Hidden,
			&c.Note,
			&c.IsSystem,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.Budgeted,
			&c.Activity,
			&c.Balance,
		)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (r *categoryRepo) GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error) {
	row := r.db.QueryRow(
		ctx, `
			SELECT 
				categories.id,
				categories.name,
				categories.budget_id,
				categories.category_group_id,
				categories.hidden,
				categories.note,
				categories.is_system,
				categories.created_at,
				categories.updated_at,
				COALESCE(
					json_object_agg(monthly_budgets.month, monthly_budgets.budgeted)
					FILTER (WHERE monthly_budgets.month IS NOT NULL), '{}'
				) AS budgeted,
				COALESCE(
					(
						SELECT json_object_agg(tx.month, tx.sum)
						FROM (
							SELECT
								TO_CHAR(date_trunc('month', transactions.date::date), 'YYYY-MM') AS month,
								SUM(transactions.amount) AS sum
							FROM transactions
							WHERE transactions.category_id = categories.id AND transactions.deleted = FALSE
							GROUP BY month
						) AS tx
					), '{}'
				) AS activity,
				COALESCE(
					json_object_agg(monthly_budgets.month, monthly_budgets.carryover_balance) 
					FILTER (WHERE monthly_budgets.month IS NOT NULL), '{}'
				) AS balance
			FROM categories 
		  LEFT JOIN monthly_budgets ON categories.id = monthly_budgets.category_id
			WHERE categories.budget_id = $1 AND categories.deleted = FALSE AND categories.id = $2
		  GROUP BY categories.id
		`, budgetId, id,
	)

	var c model.Category
	err := row.Scan(
		&c.ID,
		&c.Name,
		&c.BudgetID,
		&c.CategoryGroupID,
		&c.Hidden,
		&c.Note,
		&c.IsSystem,
		&c.CreatedAt,
		&c.UpdatedAt,
		&c.Budgeted,
		&c.Activity,
		&c.Balance,
	)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (r *categoryRepo) GetByIdSimplified(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error) {
	var c model.Category
	err := r.db.QueryRow(
		ctx, `
		  SELECT id, name, budget_id, category_group_id, hidden, note, is_system, created_at, updated_at
		  FROM categories
		  WHERE id = $1 AND budget_id = $2
		`, id, budgetId,
	).Scan(&c.ID, &c.Name, &c.BudgetID, &c.CategoryGroupID, &c.Hidden, &c.Note, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *categoryRepo) GetByIdSimplifiedTx(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error) {
	var c model.Category
	err := tx.QueryRow(
		ctx, `
		  SELECT id, name, budget_id, category_group_id, hidden, note, is_system, created_at, updated_at
		  FROM categories
		  WHERE id = $1 AND budget_id = $2
		`, id, budgetId,
	).Scan(&c.ID, &c.Name, &c.BudgetID, &c.CategoryGroupID, &c.Hidden, &c.Note, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt)
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

// Charit Bhatt
// 405 Philip Blvd Apt 302
// Lawrenceville, GA
// 30046
