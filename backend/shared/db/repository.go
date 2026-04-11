package db

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

// Executor returns the transaction if non-nil, otherwise the pool.
func (r *BaseRepository) Executor(tx pgx.Tx) DBTX {
	if tx != nil {
		return tx
	}
	return r.DB
}
