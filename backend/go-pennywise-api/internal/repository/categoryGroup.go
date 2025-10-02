package repository

import (
	"context"
	"encoding/json"
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
		ctx, `
			SELECT
				cg.id,
				cg.name,
				cg.budget_id,
				cg.hidden,
				cg.is_system,
				cg.created_at,
				cg.updated_at,

				-- Group-level budgeted (sum per month for all categories in group)
				COALESCE(
					(
						SELECT json_object_agg(month, sum_budgeted)
						FROM (
							SELECT mb.month, SUM(mb.budgeted) AS sum_budgeted
							FROM monthly_budgets mb
							JOIN categories c ON c.id = mb.category_id
							WHERE c.category_group_id = cg.id AND c.deleted = FALSE AND c.hidden = FALSE AND c.is_system = FALSE
							GROUP BY mb.month
						) t
					),
					'{}'
				) AS budgeted,

				-- Group-level activity (sum per month)
				COALESCE(
					(
						SELECT json_object_agg(month, sum_activity)
						FROM (
							SELECT
								TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
								SUM(t.amount) AS sum_activity
							FROM transactions t
							JOIN categories c ON c.id = t.category_id
							WHERE c.category_group_id = cg.id AND c.deleted = FALSE AND c.hidden = FALSE AND c.is_system = FALSE
							GROUP BY month
						) a
					),
					'{}'
				) AS activity,

				-- Group-level balance (sum per month)
				COALESCE(
					(
						SELECT json_object_agg(month, sum_balance)
						FROM (
							SELECT mb2.month, SUM(mb2.carryover_balance) AS sum_balance
							FROM monthly_budgets mb2
							JOIN categories c ON c.id = mb2.category_id
							WHERE c.category_group_id = cg.id AND c.deleted = FALSE AND c.hidden = FALSE AND c.is_system = FALSE
							GROUP BY mb2.month
						) b
					),
					'{}'
				) AS balance,

				COALESCE(
					json_agg(category_json) FILTER (WHERE category_json.id IS NOT NULL), '[]'
				) AS categories
			FROM category_groups cg
			LEFT JOIN LATERAL (
				SELECT
						-- camelCase keys to match the Categories struct
						json_build_object(
							'id', c.id,
							'name', c.name,
							'budgetId', c.budget_id,
							'categoryGroupId', c.category_group_id,
							'note', c.note,
							'hidden', c.hidden,
							'isSystem', c.is_system,
							'createdAt', c.created_at,
							'updatedAt', c.updated_at,
							'budgeted', COALESCE(
							(SELECT json_object_agg(mb.month, mb.budgeted)
								FROM monthly_budgets mb
								WHERE mb.category_id = c.id
							), '{}'
					),
					'activity', COALESCE(
						(SELECT json_object_agg(tx.month, tx.sum)
							FROM (
								SELECT
									TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
									SUM(t.amount) AS sum
								FROM transactions t
								WHERE t.category_id = c.id AND t.deleted = FALSE
								GROUP BY month
							) tx
						), '{}'
					),
					'balance', COALESCE(
						(SELECT json_object_agg(mb2.month, mb2.carryover_balance)
							FROM monthly_budgets mb2
							WHERE mb2.category_id = c.id
						), '{}'
					)
						)::jsonb AS category_json,
						c.id
				FROM categories c
				WHERE c.category_group_id = cg.id AND c.deleted = FALSE AND c.hidden = FALSE AND c.is_system = FALSE
			) category_json ON TRUE
			WHERE cg.budget_id = $1 AND cg.deleted = FALSE
			GROUP BY cg.id

			UNION ALL

			SELECT
				'00000000-0000-0000-0000-000000000000'::uuid AS id,
				'Hidden' AS name,
				$1 AS budget_id,
				TRUE AS hidden,
				FALSE AS is_system,
				now() AS created_at, -- or NULL/DEFAULT as appropriate
				now() AS updated_at,

					-- Group-level budgeted (sum per month for all categories in group)
					COALESCE(
						(
							SELECT json_object_agg(month, sum_budgeted)
							FROM (
								SELECT mb.month, SUM(mb.budgeted) AS sum_budgeted
						FROM monthly_budgets mb
						JOIN categories c ON c.id = mb.category_id
						WHERE c.hidden = TRUE AND c.deleted = FALSE 
						GROUP BY mb.month
							) t
						),
						'{}'
					) AS budgeted,

					-- Group-level activity (sum per month)
					COALESCE(
					(
						SELECT json_object_agg(month, sum_activity)
						FROM (
							SELECT
								TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
								SUM(t.amount) AS sum_activity
							FROM transactions t
							JOIN categories c ON c.id = t.category_id
							WHERE c.hidden = TRUE AND c.deleted = FALSE
							GROUP BY month
						) a
					),
					'{}'
				) AS activity,

					-- Group-level balance (sum per month)
				COALESCE(
					(
						SELECT json_object_agg(month, sum_balance)
						FROM (
							SELECT mb2.month, SUM(mb2.carryover_balance) AS sum_balance
							FROM monthly_budgets mb2
							JOIN categories c ON c.id = mb2.category_id
							WHERE c.hidden = TRUE AND c.deleted = FALSE
							GROUP BY mb2.month
						) b
					),
					'{}'
				) AS balance,
				
				COALESCE(
					(
						SELECT json_agg(category_json) 
						FROM (
							SELECT
								-- camelCase keys to match the Categories struct
								json_build_object(
								'id', c.id,
								'name', c.name,
								'budgetId', c.budget_id,
								'categoryGroupId', c.category_group_id,
								'note', c.note,
								'hidden', c.hidden,
								'isSystem', c.is_system,
								'createdAt', c.created_at,
								'updatedAt', c.updated_at,
								'budgeted', COALESCE(
									(SELECT json_object_agg(mb.month, mb.budgeted)
										FROM monthly_budgets mb
										WHERE mb.category_id = c.id
									), '{}'
								),
								'activity', COALESCE(
									(SELECT json_object_agg(tx.month, tx.sum)
										FROM (
											SELECT
												TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
												SUM(t.amount) AS sum
											FROM transactions t
											WHERE t.category_id = c.id AND t.deleted = FALSE
											GROUP BY month
										) tx
									), '{}'
								),
								'balance', COALESCE(
									(SELECT json_object_agg(mb2.month, mb2.carryover_balance)
										FROM monthly_budgets mb2
										WHERE mb2.category_id = c.id
									), '{}'
								)
								)::jsonb AS category_json
							FROM categories c
							WHERE c.budget_id = $1 AND c.hidden = TRUE AND c.deleted = FALSE
						) hidden_categories
					), 
					'[]'
				) AS categories;
		`, budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []model.CategoryGroup
	for rows.Next() {
		var g model.CategoryGroup
		var categoriesJSON []byte
		err := rows.Scan(
			&g.ID,
			&g.Name,
			&g.BudgetID,
			&g.Hidden,
			&g.IsSystem,
			&g.CreatedAt,
			&g.UpdatedAt,
			&g.Budgeted,
			&g.Activity,
			&g.Balance,
			&categoriesJSON,
		)
		if err != nil {
			return nil, err
		}
		if len(categoriesJSON) > 0 {
			err = json.Unmarshal(categoriesJSON, &g.Categories)
			if err != nil {
				return nil, err
			}
		} else {
			g.Categories = []model.Category{}
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
