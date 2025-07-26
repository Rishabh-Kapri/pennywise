package db

import (
	"context"
	"log"

	"pennywise-api/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect() *pgxpool.Pool {
	config := config.Load()
	dbpool, err := pgxpool.New(context.Background(), config.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	log.Printf("Database connection opened to %v\n", config.DatabaseURL)

	return dbpool
}
