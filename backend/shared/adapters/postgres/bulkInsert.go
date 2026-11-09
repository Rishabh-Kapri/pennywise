package postgres

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type bulkInserter struct {
	BaseRepository
}

// NewBulkInserter returns a repository.BulkInserter backed by Postgres COPY.
func NewBulkInserter(pool *pgxpool.Pool) repository.BulkInserter {
	return &bulkInserter{BaseRepository: NewBaseRepository(pool)}
}

type copier interface {
	CopyFrom(ctx context.Context, table pgx.Identifier, columns []string, src pgx.CopyFromSource) (int64, error)
}

func (b *bulkInserter) BulkInsert(ctx context.Context, table string, columns []string, rows [][]any) (int64, error) {
	var conn copier = b.DB
	if tx := txFromContext(ctx); tx != nil {
		conn = tx
	}
	return conn.CopyFrom(ctx, pgx.Identifier{table}, columns, pgx.CopyFromRows(rows))
}
