package tools

import (
	"context"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// psql is the shared statement builder. Postgres uses $N placeholders; squirrel
// defaults to ?, which pgx would reject.
var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// entityFilters are the optional name filters the query tools accept.
//
// Defined once and shared, so get_spending_summary and get_top_transactions
// cannot drift apart. A filter honored by one tool and silently dropped by the
// other is how a tagged question came back with untagged totals.
type entityFilters struct {
	categoryName string
	payeeName    string
	tagName      string
}

func newEntityFilters(categoryName, payeeName, tagName string) entityFilters {
	return entityFilters{
		categoryName: strings.TrimSpace(categoryName),
		payeeName:    strings.TrimSpace(payeeName),
		tagName:      strings.TrimSpace(tagName),
	}
}

// apply adds a predicate per non-empty filter. An unset filter contributes no
// SQL at all, rather than the "$n = ” OR ..." short-circuit a fixed query
// string needs — which is the reason these are built rather than concatenated.
//
// Each predicate is an EXISTS subquery keyed off t, not a condition on a joined
// alias, so the same filter works on any query aliasing transactions as t
// regardless of which joins that query happens to have.
func (f entityFilters) apply(query sq.SelectBuilder) sq.SelectBuilder {
	if f.categoryName != "" {
		query = query.Where(sq.Expr(
			`EXISTS (SELECT 1 FROM categories fc
			         WHERE fc.id = t.category_id
			           AND fc.budget_id = t.budget_id
			           AND fc.name ILIKE ?)`,
			"%"+f.categoryName+"%",
		))
	}
	if f.payeeName != "" {
		query = query.Where(sq.Expr(
			`EXISTS (SELECT 1 FROM payees fp
			         WHERE fp.id = t.payee_id
			           AND fp.budget_id = t.budget_id
			           AND fp.name ILIKE ?)`,
			"%"+f.payeeName+"%",
		))
	}
	// tag_ids is a UUID[] column on transactions, not a join table.
	if f.tagName != "" {
		query = query.Where(sq.Expr(
			`EXISTS (SELECT 1 FROM tags ftg
			         WHERE ftg.id = ANY(t.tag_ids)
			           AND ftg.budget_id = t.budget_id
			           AND ftg.deleted = FALSE
			           AND ftg.name ILIKE ?)`,
			"%"+f.tagName+"%",
		))
	}
	return query
}

func (f entityFilters) any() bool {
	return f.categoryName != "" || f.payeeName != "" || f.tagName != ""
}

// applied reports the filters actually in force, echoed back in tool results so
// the model states the scope of a number instead of assuming it got what it
// asked for.
func (f entityFilters) applied() map[string]string {
	if !f.any() {
		return nil
	}

	out := map[string]string{}
	for key, value := range map[string]string{
		"categoryName": f.categoryName,
		"payeeName":    f.payeeName,
		"tagName":      f.tagName,
	} {
		if value != "" {
			out[key] = value
		}
	}
	return out
}

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
