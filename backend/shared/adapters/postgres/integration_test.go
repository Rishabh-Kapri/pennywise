package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	sharedsql "github.com/Rishabh-Kapri/pennywise/backend/shared/migrations/sql"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Integration tests run against a real Postgres (pgvector image) when
// PENNYWISE_TEST_DATABASE_URL names a role allowed to CREATE DATABASE, e.g.
//
//	docker run -d --name pw-test-pg -e POSTGRES_USER=pw -e POSTGRES_PASSWORD=pw \
//	  -p 127.0.0.1:55439:5432 pgvector/pgvector:pg17
//	PENNYWISE_TEST_DATABASE_URL=postgres://pw:pw@127.0.0.1:55439/pw?sslmode=disable go test ./adapters/postgres/
//
// Each `go test` run creates its own database, applies the embedded goose
// migrations to it, and drops it at the end. Tests isolate their data by budget.
const testDatabaseURLEnv = "PENNYWISE_TEST_DATABASE_URL"

var (
	integrationOnce sync.Once
	integrationPool *pgxpool.Pool
	integrationErr  error
	integrationDrop func()
)

func TestMain(m *testing.M) {
	code := m.Run()
	if integrationDrop != nil {
		integrationDrop()
	}
	os.Exit(code)
}

// testPool returns a pool on a freshly migrated database, or skips the test
// when no test database is configured.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := os.Getenv(testDatabaseURLEnv)
	if adminURL == "" {
		t.Skipf("%s not set; skipping Postgres integration test", testDatabaseURLEnv)
	}
	integrationOnce.Do(func() {
		integrationPool, integrationDrop, integrationErr = createMigratedDatabase(adminURL)
	})
	if integrationErr != nil {
		t.Fatalf("set up test database: %v", integrationErr)
	}
	return integrationPool
}

func createMigratedDatabase(adminURL string) (*pgxpool.Pool, func(), error) {
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		return nil, nil, err
	}

	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		admin.Close()
		return nil, nil, err
	}
	name := "pw_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		return nil, nil, fmt.Errorf("create database: %w", err)
	}
	drop := func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close()
	}

	u, err := url.Parse(adminURL)
	if err != nil {
		drop()
		return nil, nil, err
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		drop()
		return nil, nil, err
	}
	if err := applyMigrations(ctx, pool); err != nil {
		pool.Close()
		drop()
		return nil, nil, err
	}
	return pool, func() {
		// Close waits for every connection to come back; a leaked one (a bug
		// under test) must not hang the run, so give up after a few seconds.
		closed := make(chan struct{})
		go func() { pool.Close(); close(closed) }()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
		}
		drop()
	}, nil
}

// applyMigrations runs the "-- +goose Up" half of every embedded migration in
// order. Exec without arguments uses the simple protocol, so a whole section
// (several statements, DO blocks) runs in one call.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := fs.ReadDir(sharedsql.Migrations, sharedsql.MigrationsDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		body, err := fs.ReadFile(sharedsql.Migrations, path.Join(sharedsql.MigrationsDir, e.Name()))
		if err != nil {
			return err
		}
		up := string(body)
		if i := strings.Index(up, "-- +goose Up"); i >= 0 {
			up = up[i+len("-- +goose Up"):]
		}
		if i := strings.Index(up, "-- +goose Down"); i >= 0 {
			up = up[:i]
		}
		if _, err := pool.Exec(ctx, up); err != nil {
			return fmt.Errorf("migration %s: %w", e.Name(), err)
		}
	}
	return nil
}

// seed is one isolated budget with the rows a transaction points at.
type seed struct {
	budgetID   uuid.UUID
	accountID  uuid.UUID
	account2ID uuid.UUID
	payeeID    uuid.UUID
	payee2ID   uuid.UUID
	categoryID uuid.UUID
	category2  uuid.UUID
}

func newSeed(t *testing.T, pool *pgxpool.Pool) seed {
	t.Helper()
	ctx := context.Background()
	s := seed{
		budgetID: uuid.New(), accountID: uuid.New(), account2ID: uuid.New(),
		payeeID: uuid.New(), payee2ID: uuid.New(), categoryID: uuid.New(), category2: uuid.New(),
	}
	groupID := uuid.New()
	stmts := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO budgets (id, name) VALUES ($1, 'Test budget')`, []any{s.budgetID}},
		{`INSERT INTO accounts (id, name, budget_id, type) VALUES ($1, 'HDFC Savings', $2, 'savings')`, []any{s.accountID, s.budgetID}},
		{`INSERT INTO accounts (id, name, budget_id, type) VALUES ($1, 'HDFC Card', $2, 'creditCard')`, []any{s.account2ID, s.budgetID}},
		{`INSERT INTO payees (id, name, budget_id) VALUES ($1, 'Blue Tokai', $2)`, []any{s.payeeID, s.budgetID}},
		{`INSERT INTO payees (id, name, budget_id) VALUES ($1, 'Swiggy', $2)`, []any{s.payee2ID, s.budgetID}},
		{`INSERT INTO category_groups (id, name, budget_id) VALUES ($1, 'Everyday', $2)`, []any{groupID, s.budgetID}},
		{`INSERT INTO categories (id, name, budget_id, category_group_id) VALUES ($1, 'Eating Out', $2, $3)`, []any{s.categoryID, s.budgetID, groupID}},
		{`INSERT INTO categories (id, name, budget_id, category_group_id) VALUES ($1, 'Groceries', $2, $3)`, []any{s.category2, s.budgetID, groupID}},
	}
	for _, st := range stmts {
		if _, err := pool.Exec(ctx, st.sql, st.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return s
}
