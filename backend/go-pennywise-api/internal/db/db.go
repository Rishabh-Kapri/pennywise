package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context) *pgxpool.Pool {
	config := config.Load()

	dbpool, err := db.ConnectWithURL(config.DatabaseURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	logger.Logger(ctx).Info("Connected to database", "url", config.DatabaseURL)

	return dbpool
}
