package postgres

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type statementPasswordRepo struct {
	BaseRepository
}

func NewStatementPasswordRepository(pool *pgxpool.Pool) repository.StatementPasswordRepository {
	return &statementPasswordRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *statementPasswordRepo) Set(ctx context.Context, budgetID, accountID uuid.UUID, sealed []byte) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		INSERT INTO account_statement_passwords (account_id, budget_id, sealed)
		SELECT id, budget_id, $3 FROM accounts WHERE id = $2 AND budget_id = $1
		ON CONFLICT (account_id) DO UPDATE SET sealed = EXCLUDED.sealed, updated_at = now()
	`, budgetID, accountID, sealed)
	return err
}

func (r *statementPasswordRepo) Delete(ctx context.Context, budgetID, accountID uuid.UUID) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		DELETE FROM account_statement_passwords WHERE budget_id = $1 AND account_id = $2
	`, budgetID, accountID)
	return err
}

func (r *statementPasswordRepo) ListByBudget(ctx context.Context, budgetID uuid.UUID) (map[uuid.UUID][]byte, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT p.account_id, p.sealed
		FROM account_statement_passwords p
		JOIN accounts a ON a.id = p.account_id AND a.deleted = FALSE
		WHERE p.budget_id = $1
	`, budgetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID][]byte{}
	for rows.Next() {
		var id uuid.UUID
		var sealed []byte
		if err := rows.Scan(&id, &sealed); err != nil {
			return nil, err
		}
		out[id] = sealed
	}
	return out, rows.Err()
}
