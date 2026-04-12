package db

import (
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect() *pgxpool.Pool {
	config := config.Load()

	dbpool, err := db.ConnectWithURL(config.DatabaseURL)
	if err != nil {
		logger.Fatal(err.Error())
	}

	return dbpool
}
