package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PayeeRuleRepository interface {
	BaseRepositoryInterface
	CreatePayeeRule(ctx context.Context, tx pgx.Tx, payeeMatch model.PayeeRule) error
	FindByMatchString(ctx context.Context, budgetId uuid.UUID, matchString string) (*model.PayeeRule, error)
	FindByPayeeID(ctx context.Context, budgetId uuid.UUID, payeeId uuid.UUID) ([]model.PayeeRuleDetails, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payeeRule model.PayeeRule) error
	DeleteByID(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}

type payeeRuleRepo struct {
	BaseRepository
}

func NewPayeeRuleRepository(pool *pgxpool.Pool) PayeeRuleRepository {
	return &payeeRuleRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *payeeRuleRepo) CreatePayeeRule(ctx context.Context, tx pgx.Tx, payeeMatch model.PayeeRule) error {
	_, err := r.Executor(tx).Exec(
		ctx, `
		INSERT INTO payee_rules (budget_id, payee_id, category_id, match_string, match_type, deleted) 
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, ''), 'EXACT')::payee_match_type, FALSE)
		ON CONFLICT (budget_id, match_string)
		DO UPDATE SET
		  payee_id = EXCLUDED.payee_id,
		  category_id = EXCLUDED.category_id,
		  match_type = EXCLUDED.match_type,
		  deleted = FALSE,
		  updated_at = NOW()
		`,
		payeeMatch.BudgetID,
		payeeMatch.PayeeID,
		payeeMatch.CategoryID,
		payeeMatch.MatchString,
		payeeMatch.MatchType,
	)
	return err
}

func (r *payeeRuleRepo) FindByMatchString(ctx context.Context, budgetId uuid.UUID, matchString string) (*model.PayeeRule, error) {
	var payeeMatch model.PayeeRule
	err := r.Executor(nil).QueryRow(
		ctx, `
		  SELECT id, budget_id, payee_id, category_id, match_string, match_type, created_at, updated_at
		  FROM payee_rules
		  WHERE budget_id = $1
		    AND deleted = FALSE
		    AND (
		      (match_type = 'EXACT' AND match_string = $2)
		      OR
		      (match_type = 'PATTERN' AND $2 ILIKE match_string)
		    )
		  ORDER BY match_type ASC
		  LIMIT 1`,
		budgetId, matchString,
	).Scan(
		&payeeMatch.ID,
		&payeeMatch.BudgetID,
		&payeeMatch.PayeeID,
		&payeeMatch.CategoryID,
		&payeeMatch.MatchString,
		&payeeMatch.MatchType,
		&payeeMatch.CreatedAt,
		&payeeMatch.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &payeeMatch, nil
}

func (r *payeeRuleRepo) FindByPayeeID(ctx context.Context, budgetId uuid.UUID, payeeId uuid.UUID) ([]model.PayeeRuleDetails, error) {
	rows, err := r.Executor(nil).Query(
		ctx, `
		  SELECT pr.id, pr.budget_id, pr.payee_id, pr.category_id, c.name, pr.match_string, pr.match_type, pr.created_at, pr.updated_at
		  FROM payee_rules pr
		  LEFT JOIN categories c ON c.id = pr.category_id AND c.budget_id = pr.budget_id AND c.deleted = FALSE
		  WHERE pr.budget_id = $1
		    AND pr.payee_id = $2
		    AND pr.deleted = FALSE
		  ORDER BY pr.match_string ASC`,
		budgetId, payeeId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []model.PayeeRuleDetails
	for rows.Next() {
		var rule model.PayeeRuleDetails
		if err := rows.Scan(
			&rule.ID,
			&rule.BudgetID,
			&rule.PayeeID,
			&rule.CategoryID,
			&rule.CategoryName,
			&rule.MatchString,
			&rule.MatchType,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

func (r *payeeRuleRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payeeRule model.PayeeRule) error {
	_, err := r.Executor(nil).Exec(
		ctx, `
		  UPDATE payee_rules
		  SET category_id = $1,
		      match_string = $2,
		      match_type = COALESCE(NULLIF($3, ''), 'EXACT')::payee_match_type,
		      updated_at = NOW()
		  WHERE id = $4
		    AND budget_id = $5
		    AND payee_id = $6
		    AND deleted = FALSE`,
		payeeRule.CategoryID,
		payeeRule.MatchString,
		payeeRule.MatchType,
		id,
		budgetId,
		payeeRule.PayeeID,
	)
	return err
}

func (r *payeeRuleRepo) DeleteByID(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	_, err := r.Executor(nil).Exec(
		ctx, `
		  UPDATE payee_rules
		  SET deleted = TRUE,
		      updated_at = NOW()
		  WHERE id = $1
		    AND budget_id = $2
		    AND deleted = FALSE`,
		id,
		budgetId,
	)
	return err
}
