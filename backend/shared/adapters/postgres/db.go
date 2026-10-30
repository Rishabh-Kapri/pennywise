package postgres

import (
	"context"
	"log"
	"os"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func Connect() *pgxpool.Pool {
	_ = godotenv.Load(".env")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	dbpool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}

	log.Printf("Database connection opened to %v\n", databaseURL)
	return dbpool
}

func ConnectWithURL(databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "database URL cannot be empty")
	}

	dbpool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, errs.Wrap(errs.CodeHTTPClientError, "Unable to connect to database", err)
	}

	if err := dbpool.Ping(context.Background()); err != nil {
		return nil, errs.Wrap(errs.CodeHTTPClientError, "Unable to ping database", err)
	}

	return dbpool, nil
}

// vectorLiteral renders an embedding as a pgvector literal.
func vectorLiteral(vec []float64) string {
	return model.FormatVector(vec)
}
