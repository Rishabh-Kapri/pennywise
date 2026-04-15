package main

import (
	"context"
	"database/sql"
	"flag"
	"log"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/db/migrations"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	var dir string
	flag.StringVar(&dir, "dir", ".", "directory with migration files")
	flag.Parse()

	args := flag.Args()

	if len(args) < 1 {
		args = []string{"up"} // Default command
	}

	command := args[0]
	var arguments []string
	if len(args) > 1 {
		arguments = args[1:]
	}

	cfg := config.Load()

	// Connect to database using pgx stdlib
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open db connection: %v\n", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v\n", err)
	}

	goose.SetDialect("postgres")
	goose.SetBaseFS(migrations.EmbedMigrations)

	if command == "baseline" {
		log.Println("Baselining database to skip init schema and seed data (versions 1-6)...")
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS goose_db_version (
				id serial PRIMARY KEY,
				version_id bigint NOT NULL,
				is_applied boolean NOT NULL,
				tstamp timestamp DEFAULT now()
			);
		`)
		if err != nil {
			log.Fatalf("error creating goose_db_version: %v", err)
		}

		for i := 1; i <= 6; i++ {
			_, err = db.Exec("INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true) ON CONFLICT DO NOTHING", i)
			if err != nil {
				log.Printf("Warning: checking baseline insertion for version %d (might already exist)", i)
			}
		}
		log.Println("Database successfully baselined. Migrations 1-6 are marked as applied.")
		return
	}

	if command == "help" {
		log.Println(`
Pennywise Database Migrations (via pressly/goose)

Commands:
  up                   Migrate the DB to the most recent version available
  up-by-one            Migrate the DB up by 1
  up-to VERSION        Migrate the DB to a specific VERSION
  down                 Roll back the version by 1
  down-to VERSION      Roll back to a specific VERSION
  redo                 Re-run the latest migration
  reset                Roll back all migrations
  status               Dump the migration status for the current DB
  version              Print the current version of the database
  create NAME [sql|go] Creates new migration file with the current timestamp
  baseline             (Custom) Marks initial JSON seedings (1-6) as applied
`)
		return
	}

	log.Printf("running goose command: %q\n", command)

	if err := goose.RunContext(context.Background(), command, db, dir, arguments...); err != nil {
		log.Fatalf("goose run error: %v\n", err)
	}

	log.Println("migration completed successfully")
}
