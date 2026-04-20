package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PayeeMatchRepository interface {
	BaseRepositoryInterface
	CreatePayeeMatch(ctx context.Context, tx pgx.Tx, payeeMatch model.PayeeMatch) error
	FindByMatchString(ctx context.Context, budgetId uuid.UUID, matchString string) (*model.PayeeMatch, error)
}

type payeeMatchRepo struct {
	BaseRepository
}

func NewPayeeMatchRepository(pool *pgxpool.Pool) PayeeMatchRepository {
	return &payeeMatchRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *payeeMatchRepo) CreatePayeeMatch(ctx context.Context, tx pgx.Tx, payeeMatch model.PayeeMatch) error {
	_, err := r.Executor(tx).Exec(
		ctx,
		`INSERT INTO payee_matches (
		   budget_id, payee_id, match_string, created_at, updated_at
		) VALUES ($1, $2, $3, NOW(), NOW())`,
		payeeMatch.BudgetID,
		payeeMatch.PayeeID,
		payeeMatch.MatchString,
	)
	return err
}

func (r *payeeMatchRepo) FindByMatchString(ctx context.Context, budgetId uuid.UUID, matchString string) (*model.PayeeMatch, error) {
	var payeeMatch model.PayeeMatch
	err := r.Executor(nil).QueryRow(
		ctx, `
		  SELECT id, budget_id, payee_id, match_string, created_at, updated_at
		  FROM payee_matches
		  WHERE budget_id = $1 AND match_string = $2`,
		budgetId, matchString,
	).Scan(
		&payeeMatch.ID,
		&payeeMatch.BudgetID,
		&payeeMatch.PayeeID,
		&payeeMatch.MatchString,
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
