package tools

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultToolQueryTimeout bounds any single agent-issued query. The model can
// generate an accidental cross join, and without a server-side timeout that ties
// up a connection long after the chat request is gone.
const defaultToolQueryTimeout = 5 * time.Second

// withBudgetScopedTx runs fn inside a read-only transaction that carries the
// caller's budget in app.budget_id.
//
// Two things are enforced here rather than in the prompt: Postgres rejects
// writes outright in a read-only transaction, and row-level security policies
// keyed on app.budget_id confine reads to a single budget. A model mistake or a
// prompt injection therefore cannot escape the budget, whatever SQL it produces.
//
// statement_timeout and app.budget_id are both applied with set_config(...,
// true) so they are transaction-local and can take bind parameters — SET LOCAL
// cannot, and interpolating into it would reintroduce an injection point.
func withBudgetScopedTx(
	ctx context.Context,
	pool *pgxpool.Pool,
	budgetID uuid.UUID,
	fn func(pgx.Tx) error,
) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	// Read-only: nothing to commit, and rollback releases the connection.
	defer func() { _ = tx.Rollback(ctx) }()

	timeoutMillis := strconv.FormatInt(defaultToolQueryTimeout.Milliseconds(), 10)
	if _, err := tx.Exec(ctx, "SELECT set_config('statement_timeout', $1, true)", timeoutMillis); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.budget_id', $1, true)", budgetID.String()); err != nil {
		return err
	}

	return fn(tx)
}
