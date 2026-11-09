package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX provides a common interface for both pgxpool.Pool and pgx.Tx queries.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// BaseRepository provides common database operations for all repositories.
type BaseRepository struct {
	DB *pgxpool.Pool
}

func NewBaseRepository(db *pgxpool.Pool) BaseRepository {
	return BaseRepository{DB: db}
}

func (r *BaseRepository) GetDB() *pgxpool.Pool {
	return r.DB
}

// Executor returns the transaction carried in ctx (see DBTxn.Do), otherwise
// the pool. Single-row lookups that find nothing return an error matching
// both ports.ErrNotFound and pgx.ErrNoRows.
func (r *BaseRepository) Executor(ctx context.Context) DBTX {
	if tx := txFromContext(ctx); tx != nil {
		return notFoundExecutor{tx}
	}
	return notFoundExecutor{r.DB}
}

// InTx runs fn in the transaction already in ctx, or in a new one. Use it for
// repository methods that issue several statements that must apply together.
func (r *BaseRepository) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return NewDBTxn(r.DB).Do(ctx, fn)
}
