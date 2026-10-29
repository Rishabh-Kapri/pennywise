package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math"
	"strings"
	"text/template"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	pwsql "github.com/Rishabh-Kapri/pennywise/backend/shared/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// legacyFinalVersion is the last version of the pre-squash migration history
// (29 files, since replaced by the ones in backend/shared/sql/migrations).
const legacyFinalVersion = 29

const helpText = `
Pennywise Database Migrations (via pressly/goose)

Migrations live in backend/shared/sql/migrations and are embedded in the binary.

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
  create NAME          Create the next sequential migration file in the shared sql dir
                       (-dir is the on-disk directory for this command)
  baseline             One-off: adopt the squashed history on a DB that was built by the
                       old 29 migrations. Marks every migration as applied without running it.
`

// New migrations start with the section markers only; goose needs
// StatementBegin/End solely around statements containing semicolons (DO blocks, functions).
var createTemplate = template.Must(template.New("migration").Parse(`-- +goose Up

-- +goose Down
`))

func main() {
	var dir string
	flag.StringVar(&dir, "dir", "", "migrations directory: inside the embedded FS (default \""+
		pwsql.MigrationsDir+"\"), or on disk for create (default \"../shared/sql/"+pwsql.MigrationsDir+"\")")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		args = []string{"up"} // Default command
	}
	command, arguments := args[0], args[1:]

	switch command {
	case "help":
		log.Print(helpText)
		return
	case "create":
		if len(arguments) != 1 {
			log.Fatalf("usage: create NAME")
		}
		if dir == "" {
			dir = "../shared/sql/" + pwsql.MigrationsDir
		}
		goose.SetSequential(true)
		if err := goose.CreateWithTemplate(nil, dir, createTemplate, arguments[0], "sql"); err != nil {
			log.Fatalf("create migration: %v\n", err)
		}
		return
	}

	if dir == "" {
		dir = pwsql.MigrationsDir
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
	goose.SetBaseFS(pwsql.Migrations)

	if command == "baseline" {
		if err := baseline(context.Background(), db, dir); err != nil {
			log.Fatalf("baseline: %v\n", err)
		}
		return
	}

	log.Printf("running goose command: %q\n", command)

	if err := goose.RunContext(context.Background(), command, db, dir, arguments...); err != nil {
		log.Fatalf("goose run error: %v\n", err)
	}

	log.Println("migration completed successfully")
}

// baseline adopts the squashed history on a database whose schema was built by
// the old 29-file history: it replaces goose_db_version with one applied row per
// current migration, without running any of them. Goose would otherwise see
// versions 10..29 applied that no longer exist, and refuse every future
// migration numbered below 29 as "missing".
//
// It only accepts a database sitting exactly at the old final version; adopting
// anything older would claim tables that were never created.
func baseline(ctx context.Context, db *sql.DB, dir string) error {
	var hasSchema bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.budgets') IS NOT NULL`).Scan(&hasSchema); err != nil {
		return err
	}
	if !hasSchema {
		return fmt.Errorf("no existing schema to adopt; run `up` against an empty database instead")
	}

	if _, err := goose.EnsureDBVersion(db); err != nil {
		return err
	}
	var current int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied`).Scan(&current); err != nil {
		return err
	}
	if current != legacyFinalVersion {
		return fmt.Errorf("database is at version %d; baseline only adopts a database at the old final version %d "+
			"(bring it there with the old migrations first, or recreate it and run `up`)", current, legacyFinalVersion)
	}

	collected, err := goose.CollectMigrations(dir, 0, math.MaxInt64)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Version 0 is goose's own initial row.
	if _, err := tx.ExecContext(ctx, `DELETE FROM goose_db_version WHERE version_id <> 0`); err != nil {
		return err
	}
	versions := make([]string, 0, len(collected))
	for _, m := range collected {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true)`, m.Version); err != nil {
			return err
		}
		versions = append(versions, fmt.Sprint(m.Version))
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("baselined: marked versions %s as applied (was %d)\n", strings.Join(versions, ", "), current)
	return nil
}
