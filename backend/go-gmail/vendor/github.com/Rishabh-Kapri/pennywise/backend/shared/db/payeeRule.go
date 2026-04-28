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
		INSERT INTO payee_rules (budget_id, payee_id, category_id, match_string) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (budget_id, match_string)
		DO UPDATE SET
		  payee_id = EXCLUDED.payee_id,
		  category_id = EXCLUDED.category_id,
		  updated_at = NOW()
		`,
		payeeMatch.BudgetID,
		payeeMatch.PayeeID,
		payeeMatch.CategoryID,
		payeeMatch.MatchString,
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
