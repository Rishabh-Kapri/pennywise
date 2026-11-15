package postgres

import (
	"context"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectWithURL(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "database URL cannot be empty")
	}

	dbpool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, errs.Wrap(errs.CodeHTTPClientError, "Unable to connect to database", err)
	}

	if err := dbpool.Ping(ctx); err != nil {
		return nil, errs.Wrap(errs.CodeHTTPClientError, "Unable to ping database", err)
	}

	return dbpool, nil
}
