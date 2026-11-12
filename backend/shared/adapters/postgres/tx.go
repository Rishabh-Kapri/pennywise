package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

func contextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func txFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txKey{}).(pgx.Tx)
	return tx
}

type dbTxn struct {
	pool *pgxpool.Pool
}

// NewDBTxn returns the Postgres implementation of repository.DBTxn.
func NewDBTxn(pool *pgxpool.Pool) repository.DBTxn {
	return &dbTxn{pool: pool}
}

func (t *dbTxn) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if txFromContext(ctx) != nil {
		return fn(ctx) // already inside a transaction: join it
	}
	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // no-op after Commit
	if err := fn(contextWithTx(ctx, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// errNoRows is what repositories return when an explicit lookup finds nothing.
var errNoRows = notFound(pgx.ErrNoRows)

// notFoundError is errNoRows for a write that matched no row, named for what
// was being looked for.
func notFoundError(what string) error {
	return fmt.Errorf("%s not found: %w", what, errNoRows)
}

// notFound wraps err so callers can match it with errors.Is against
// repository.ErrNotFound; it still matches pgx.ErrNoRows too.
func notFound(err error) error {
	return fmt.Errorf("%w: %w", repository.ErrNotFound, err)
}

// notFoundExecutor maps pgx.ErrNoRows from QueryRow().Scan to notFound.
type notFoundExecutor struct {
	DBTX
}

func (e notFoundExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return notFoundRow{e.DBTX.QueryRow(ctx, sql, args...)}
}

type notFoundRow struct {
	pgx.Row
}

func (r notFoundRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound(err)
	}
	return err
}
