package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// common interface for both pgxpool.Pool and pgx.Tx queries
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// provides common database operations for all repositories
type BaseRepository interface {
	GetPgxTx(ctx context.Context) (pgx.Tx, error)
}

type baseRepository struct {
	db *pgxpool.Pool
}

func NewBaseRepository(db *pgxpool.Pool) baseRepository {
	return baseRepository{db: db}
}

func (r *baseRepository) GetDB() *pgxpool.Pool {
	return r.db
}

func (r *baseRepository) GetPgxTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.BeginTx(ctx, pgx.TxOptions{})
}

func (r *baseRepository) Executor(tx pgx.Tx) DBTX {
	if tx != nil {
		return tx
	}
	return r.db
}
