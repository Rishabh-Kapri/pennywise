package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"pennywise-api/internal/db"
	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrator defines the interface for running a data migration step.
type Migrator interface {
	Run(ctx context.Context, pool *pgxpool.Pool, path string) error
	Name() string
}

type InflowCategory struct {
	model.Category
}

type genericMigrator[T any] struct {
	TableName string
	InsertFn  func(ctx context.Context, pool *pgxpool.Pool, data []T) error
}

func (m genericMigrator[T]) Run(ctx context.Context, pool *pgxpool.Pool, path string) error {
	return runMigration(ctx, pool, m, path)
}

func (m genericMigrator[T]) Name() string {
	return m.TableName
}

func uuidOrNull(id *uuid.UUID) interface{} {
	if id != nil && *id == uuid.Nil {
		return nil
	}
	return id
}

func loadFileData[T any](path string) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", path, err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", path, err)
	}
	var data []T
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling file %s: %w", path, err)
	}
	log.Printf("Loaded total data for file %v: %v\n", path, len(data))

	return data, nil
}

func runMigration[T any](ctx context.Context, pool *pgxpool.Pool, m genericMigrator[T], path string) error {
	var data []T
	if path != "" {
		returnedData, err := loadFileData[T](path)
		if err != nil {
			return fmt.Errorf("runMigration: error loading file: %w", err)
		}
		data = returnedData
	}
	if err := m.InsertFn(ctx, pool, data); err != nil {
		return fmt.Errorf("runMigration: error inserting data for %s: %w", path, err)
	}
	log.Printf("Migrated %s: %d items", m.TableName, len(data))
	return nil
}

// createSchema creates all database tables, constraints, and indexes in dependency order.
// It is idempotent and safe to run multiple times.
func createSchema(ctx context.Context, pool *pgxpool.Pool) error {
	tables := []struct {
		name  string
		query string
	}{
		{"schema_migrations", `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`},
		{"auth_users", `
			CREATE TABLE IF NOT EXISTS auth_users (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				google_id TEXT NOT NULL UNIQUE,
				email TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				picture TEXT,
				token_version INTEGER NOT NULL DEFAULT 1,
				refresh_token_hash TEXT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				deleted BOOLEAN DEFAULT false
			)`},
		{"budgets", `
			CREATE TABLE IF NOT EXISTS budgets (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				user_id UUID REFERENCES auth_users(id),
				name TEXT NOT NULL,
				is_selected BOOLEAN,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				deleted BOOLEAN DEFAULT false
			)`},
		{"accounts", `
			CREATE TABLE IF NOT EXISTS accounts (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name TEXT NOT NULL,
				budget_id UUID NOT NULL REFERENCES budgets(id),
				transfer_payee_id UUID,
				type TEXT NOT NULL,
				closed BOOLEAN DEFAULT false,
				deleted BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`},
		{"payees", `
			CREATE TABLE IF NOT EXISTS payees (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name TEXT NOT NULL,
				budget_id UUID NOT NULL REFERENCES budgets(id),
				transfer_account_id UUID REFERENCES accounts(id),
				deleted BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`},
		{"users", `
			CREATE TABLE IF NOT EXISTS users (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				budget_id UUID NOT NULL REFERENCES budgets(id),
				email TEXT NOT NULL,
				history_id NUMERIC(10, 0) NOT NULL,
				gmail_refresh_token TEXT NOT NULL,
				deleted BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`},
		{"category_groups", `
			CREATE TABLE IF NOT EXISTS category_groups (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name TEXT NOT NULL,
				budget_id UUID NOT NULL REFERENCES budgets(id),
				hidden BOOLEAN DEFAULT false,
				is_system BOOLEAN DEFAULT false,
				deleted BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`},
		{"categories", `
			CREATE TABLE IF NOT EXISTS categories (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name TEXT NOT NULL,
				budget_id UUID NOT NULL REFERENCES budgets(id),
				category_group_id UUID NOT NULL REFERENCES category_groups(id),
				note TEXT,
				hidden BOOLEAN DEFAULT false,
				is_system BOOLEAN DEFAULT false,
				deleted BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`},
		{"monthly_budgets", `
			CREATE TABLE IF NOT EXISTS monthly_budgets (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				month TEXT NOT NULL,
				budget_id UUID NOT NULL REFERENCES budgets(id),
				category_id UUID NOT NULL REFERENCES categories(id),
				budgeted NUMERIC(12, 2) NOT NULL,
				carryover_balance NUMERIC(12, 2) NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`},
		{"transactions", `
			CREATE TABLE IF NOT EXISTS transactions (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				budget_id UUID NOT NULL REFERENCES budgets(id),
				date TEXT NOT NULL,
				payee_id UUID,
				category_id UUID,
				account_id UUID NOT NULL,
				note TEXT,
				amount NUMERIC(12, 2) NOT NULL,
				source TEXT,
				transfer_account_id UUID,
				transfer_transaction_id UUID,
				deleted BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`},
		{"predictions", `
			CREATE TABLE IF NOT EXISTS predictions (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				budget_id UUID NOT NULL REFERENCES budgets(id),
				transaction_id UUID NOT NULL REFERENCES transactions(id),
				email_text TEXT,
				amount NUMERIC(12, 2),
				account TEXT,
				account_prediction NUMERIC(10, 2),
				payee TEXT,
				payee_prediction NUMERIC(10, 2),
				category TEXT,
				category_prediction NUMERIC(10, 2),
				has_user_corrected BOOLEAN,
				user_corrected_account TEXT,
				user_corrected_payee TEXT,
				user_corrected_category TEXT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				deleted BOOLEAN DEFAULT false
			)`},
		{"loan_metadata", `
			CREATE TABLE IF NOT EXISTS loan_metadata (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				account_id UUID NOT NULL REFERENCES accounts(id) UNIQUE,
				interest_rate NUMERIC(6, 3) NOT NULL,
				original_balance NUMERIC(12, 2) NOT NULL,
				monthly_payment NUMERIC(12, 2) NOT NULL,
				loan_start_date TEXT NOT NULL,
				category_id UUID REFERENCES categories(id),
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				deleted BOOLEAN DEFAULT false
			)`},
		{"tags", `
			CREATE TABLE IF NOT EXISTS tags (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name TEXT NOT NULL,
				budget_id UUID NOT NULL REFERENCES budgets(id),
				color TEXT NOT NULL DEFAULT '',
				deleted BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				UNIQUE(name, budget_id)
			)`},
	}

	for _, t := range tables {
		if _, err := pool.Exec(ctx, t.query); err != nil {
			return fmt.Errorf("error creating table %s: %w", t.name, err)
		}
		log.Printf("Ensured table exists: %s", t.name)
	}

	// Add foreign key constraints idempotently using DO blocks
	constraints := []struct {
		name  string
		query string
	}{
		{"fk_transfer_payee_id", `
			DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transfer_payee_id') THEN
					ALTER TABLE accounts ADD CONSTRAINT fk_transfer_payee_id
						FOREIGN KEY (transfer_payee_id) REFERENCES payees(id);
				END IF;
			END $$`},
		{"fk_transfer_transaction_id", `
			DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transfer_transaction_id') THEN
					ALTER TABLE transactions ADD CONSTRAINT fk_transfer_transaction_id
						FOREIGN KEY (transfer_transaction_id) REFERENCES transactions(id);
				END IF;
			END $$`},
		{"fk_payee_id", `
			DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_payee_id') THEN
					ALTER TABLE transactions ADD CONSTRAINT fk_payee_id
						FOREIGN KEY (payee_id) REFERENCES payees(id);
				END IF;
			END $$`},
		{"fk_account_id", `
			DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_account_id') THEN
					ALTER TABLE transactions ADD CONSTRAINT fk_account_id
						FOREIGN KEY (account_id) REFERENCES accounts(id);
				END IF;
			END $$`},
		{"fk_category_id", `
			DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_category_id') THEN
					ALTER TABLE transactions ADD CONSTRAINT fk_category_id
						FOREIGN KEY (category_id) REFERENCES categories(id);
				END IF;
			END $$`},
		{"fk_transfer_account_id", `
			DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transfer_account_id') THEN
					ALTER TABLE transactions ADD CONSTRAINT fk_transfer_account_id
						FOREIGN KEY (transfer_account_id) REFERENCES accounts(id);
				END IF;
			END $$`},
	}

	for _, c := range constraints {
		if _, err := pool.Exec(ctx, c.query); err != nil {
			return fmt.Errorf("error adding constraint %s: %w", c.name, err)
		}
		log.Printf("Ensured constraint exists: %s", c.name)
	}

	// Create indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_auth_users_google_id ON auth_users(google_id)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_users_email ON auth_users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_budgets_user_id ON budgets(user_id)`,

		// transactions: covers GetAll, GetAllNormalized sorted listing
		`CREATE INDEX IF NOT EXISTS idx_transactions_budget_date ON transactions(budget_id, date DESC, updated_at DESC) WHERE deleted = FALSE`,
		// transactions: covers account balance subquery in accounts.GetAll
		`CREATE INDEX IF NOT EXISTS idx_transactions_account ON transactions(account_id, budget_id) WHERE deleted = FALSE`,
		// transactions: covers category activity subquery in categories.GetAll
		`CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category_id, budget_id) WHERE deleted = FALSE`,

		// monthly_budgets: covers GetByCatIdAndMonth and carryover cascading
		`CREATE INDEX IF NOT EXISTS idx_monthly_budgets_lookup ON monthly_budgets(budget_id, category_id, month)`,

		// predictions: covers GetByTxnId lookup during transaction updates
		`CREATE INDEX IF NOT EXISTS idx_predictions_txn ON predictions(budget_id, transaction_id) WHERE deleted = FALSE`,

		// payees: covers GetAll and Search by name
		`CREATE INDEX IF NOT EXISTS idx_payees_budget ON payees(budget_id) WHERE deleted = FALSE`,

		// accounts: covers GetAll listing
		`CREATE INDEX IF NOT EXISTS idx_accounts_budget ON accounts(budget_id) WHERE deleted = FALSE`,

		// categories: covers GetAll, GetByFilter listing
		`CREATE INDEX IF NOT EXISTS idx_categories_budget ON categories(budget_id) WHERE deleted = FALSE`,
	}

	for _, idx := range indexes {
		if _, err := pool.Exec(ctx, idx); err != nil {
			log.Printf("Warning: could not create index: %v", err)
		}
	}

	log.Printf("Schema creation completed successfully")
	return nil
}

// --- Migration versioning helpers ---

func isMigrationApplied(ctx context.Context, pool *pgxpool.Pool, version int) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking migration version %d: %w", version, err)
	}
	return exists, nil
}

func recordMigration(ctx context.Context, pool *pgxpool.Pool, version int, name string) error {
	_, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2) ON CONFLICT DO NOTHING`, version, name)
	if err != nil {
		return fmt.Errorf("error recording migration version %d (%s): %w", version, name, err)
	}
	return nil
}

// --- Data insertion functions (pure data operations, no DDL) ---

func insertBudgets(ctx context.Context, pool *pgxpool.Pool, data []model.Budget) error {
	batch := &pgx.Batch{}
	for _, d := range data {
		batch.Queue(
			`INSERT INTO budgets (
				id, name, is_selected, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
			d.ID, d.Name, d.IsSelected, d.CreatedAt, d.UpdatedAt,
		)
	}
	br := pool.SendBatch(ctx, batch)
	return br.Close()
}

func insertAccounts(ctx context.Context, pool *pgxpool.Pool, data []model.Account) error {
	batch := &pgx.Batch{}
	for _, d := range data {
		batch.Queue(
			`INSERT INTO accounts (
				id, name, budget_id, transfer_payee_id, type, closed, deleted, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9
			) ON CONFLICT DO NOTHING`,
			d.ID,
			d.Name,
			d.BudgetID,
			uuidOrNull(d.TransferPayeeID),
			d.Type,
			d.Closed,
			d.Deleted,
			d.CreatedAt,
			d.UpdatedAt,
		)
	}
	br := pool.SendBatch(ctx, batch)
	return br.Close()
}

func insertUsers(ctx context.Context, pool *pgxpool.Pool, data []model.User) error {
	batch := &pgx.Batch{}
	for _, d := range data {
		batch.Queue(
			`INSERT INTO users (
				id, budget_id, email, history_id, deleted, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING`,
			d.ID,
			d.BudgetID,
			d.Email,
			d.HistoryID,
			d.Deleted,
			d.CreatedAt,
			d.UpdatedAt,
		)
	}
	br := pool.SendBatch(ctx, batch)
	return br.Close()
}

func insertCategoryGroups(ctx context.Context, pool *pgxpool.Pool, data []model.CategoryGroup) error {
	batch := &pgx.Batch{}
	for _, d := range data {
		isSystem := d.Name == "Credit Card Payments"
		batch.Queue(
			`INSERT INTO category_groups (
				id, name, budget_id, hidden, is_system, deleted, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT DO NOTHING`,
			d.ID,
			d.Name,
			d.BudgetID,
			d.Hidden,
			isSystem,
			d.Deleted,
			d.CreatedAt,
			d.UpdatedAt,
		)
	}
	br := pool.SendBatch(ctx, batch)
	return br.Close()
}

func insertMonthlyBudgets(ctx context.Context, pool *pgxpool.Pool, data map[string]float32, budgetId uuid.UUID, categoryId uuid.UUID) error {
	batch := &pgx.Batch{}
	for month, budgeted := range data {
		key := strings.Split(month, "-")
		keyFloat, err := strconv.ParseFloat(key[1], 10)
		if err != nil {
			return fmt.Errorf("cannot convert month key %q to float: %w", key[1], err)
		}
		keyInt := int(keyFloat) + 1
		newKey := key[0] + "-" + fmt.Sprintf("%02d", keyInt)
		batch.Queue(`
			INSERT INTO monthly_budgets (
				budget_id, category_id, month, budgeted, carryover_balance, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			ON CONFLICT DO NOTHING
		`, budgetId, categoryId, newKey, budgeted, 0,
		)
	}
	br := pool.SendBatch(ctx, batch)
	return br.Close()
}

func insertCategories(ctx context.Context, pool *pgxpool.Pool, data []model.Category) error {
	batch := &pgx.Batch{}
	for _, d := range data {
		isSystem := d.Name == "Inflow: Ready to Assign"
		if !isSystem {
			if err := insertMonthlyBudgets(ctx, pool, d.Budgeted, d.BudgetID, d.ID); err != nil {
				return fmt.Errorf("error inserting monthly budgets for category %s: %w", d.ID, err)
			}
		}
		batch.Queue(
			`INSERT INTO categories (
				id, name, budget_id, category_group_id, note, hidden, is_system, deleted, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
			d.ID,
			d.Name,
			d.BudgetID,
			d.CategoryGroupID,
			d.Note,
			d.Hidden,
			isSystem,
			d.Deleted,
			d.CreatedAt,
			d.UpdatedAt,
		)
	}
	br := pool.SendBatch(ctx, batch)
	return br.Close()
}

func insertInflowCategory(ctx context.Context, pool *pgxpool.Pool, data []InflowCategory) error {
	inflowCat := data[0]
	_, err := pool.Exec(ctx, `
		INSERT INTO categories (
			id, name, budget_id, category_group_id, note, hidden, is_system, deleted, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
		inflowCat.ID, inflowCat.Name, inflowCat.BudgetID, inflowCat.CategoryGroupID, inflowCat.Note, false, true, false, inflowCat.CreatedAt, inflowCat.UpdatedAt,
	)
	return err
}

func insertPayees(ctx context.Context, pool *pgxpool.Pool, data []model.Payee) error {
	batch := &pgx.Batch{}
	for _, d := range data {
		batch.Queue(`
			INSERT INTO payees (
				id, name, budget_id, transfer_account_id, deleted, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING
		`,
			d.ID,
			d.Name,
			d.BudgetID,
			uuidOrNull(d.TransferAccountID),
			d.Deleted,
			d.CreatedAt,
			d.UpdatedAt,
		)
	}
	br := pool.SendBatch(ctx, batch)
	return br.Close()
}

func insertTransactions(ctx context.Context, pool *pgxpool.Pool, data []model.Transaction) error {
	batch := &pgx.Batch{}
	for _, d := range data {
		batch.Queue(
			`INSERT INTO transactions (
				id, budget_id, date, payee_id, category_id, account_id, note, amount, source, transfer_account_id, transfer_transaction_id, deleted, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
			) ON CONFLICT DO NOTHING`,
			d.ID,
			d.BudgetID,
			d.Date,
			uuidOrNull(d.PayeeID),
			uuidOrNull(d.CategoryID),
			uuidOrNull(d.AccountID),
			d.Note,
			d.Amount,
			d.Source,
			uuidOrNull(d.TransferAccountID),
			uuidOrNull(d.TransferTransactionID),
			d.Deleted,
			d.CreatedAt,
			d.UpdatedAt,
		)
	}
	br := pool.SendBatch(ctx, batch)
	return br.Close()
}

// addTagIdsColumn adds the tag_ids UUID[] column to the transactions table.
func addTagIdsColumn(ctx context.Context, pool *pgxpool.Pool, _ []struct{}) error {
	_, err := pool.Exec(ctx, `
		ALTER TABLE transactions ADD COLUMN IF NOT EXISTS tag_ids UUID[] DEFAULT '{}'
	`)
	if err != nil {
		return fmt.Errorf("error adding tag_ids column: %w", err)
	}
	log.Printf("Added tag_ids column to transactions table")
	return nil
}

// createTransactionEmbeddingsTable creates the transaction_embeddings table for pgvector-based prediction.
func createTransactionEmbeddingsTable(ctx context.Context, pool *pgxpool.Pool, _ []struct{}) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS transaction_embeddings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			budget_id UUID NOT NULL,
			embedding_text TEXT NOT NULL,
			embedding vector(1024) NOT NULL,
			payee TEXT NOT NULL,
			category TEXT NOT NULL,
			account TEXT NOT NULL,
			amount FLOAT NOT NULL,
			transaction_id UUID,
			source VARCHAR(20) NOT NULL DEFAULT 'prediction',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	if err != nil {
		return fmt.Errorf("error creating transaction_embeddings table: %w", err)
	}

	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_txn_embed_txn_id_unique
			ON transaction_embeddings(transaction_id)
			WHERE transaction_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_txn_embed_cosine
			ON transaction_embeddings
			USING ivfflat (embedding vector_cosine_ops)
			WITH (lists = 20)`,
		`CREATE INDEX IF NOT EXISTS idx_txn_embed_budget
			ON transaction_embeddings(budget_id)`,
	}
	for _, idx := range indexes {
		if _, err := pool.Exec(ctx, idx); err != nil {
			log.Printf("Warning: could not create index: %v", err)
		}
	}

	log.Printf("Created transaction_embeddings table with indexes")
	return nil
}

// cityMonthYearRe matches patterns like "Nagpur Apr 2025", "New York Jan 2024"
var cityMonthYearRe = regexp.MustCompile(`(?i)^[a-z][\w\s]+ (jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+\d{4}$`)

// isTripTag returns true if the tag prefix contains "trip" or matches "City Month Year".
func isTripTag(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "trip") || cityMonthYearRe.MatchString(name)
}

// migrateNotesToTags parses existing transaction notes that match the pattern "<tag>: <rest>"
// and creates tags + associations from them.
func migrateNotesToTags(ctx context.Context, pool *pgxpool.Pool, _ []struct{}) error {
	// Find transactions with notes containing ":" pattern
	rows, err := pool.Query(ctx, `
		SELECT id, budget_id, note FROM transactions
		WHERE deleted = FALSE AND note IS NOT NULL AND note LIKE '%:%'
		ORDER BY budget_id
	`)
	if err != nil {
		return fmt.Errorf("error querying transactions with notes: %w", err)
	}
	defer rows.Close()

	type txnNote struct {
		ID       uuid.UUID
		BudgetID uuid.UUID
		Note     string
	}
	var txnNotes []txnNote
	for rows.Next() {
		var t txnNote
		if err := rows.Scan(&t.ID, &t.BudgetID, &t.Note); err != nil {
			return fmt.Errorf("error scanning transaction note: %w", err)
		}
		txnNotes = append(txnNotes, t)
	}

	if len(txnNotes) == 0 {
		log.Printf("No transactions with colon-separated notes found")
		return nil
	}

	// Track created tags per budget to avoid duplicates
	tagCache := make(map[string]uuid.UUID) // key: "budgetId:tagName"

	for _, t := range txnNotes {
		parts := strings.SplitN(t.Note, ":", 2)
		if len(parts) != 2 {
			continue
		}
		tagName := strings.TrimSpace(parts[0])
		remainingNote := strings.TrimSpace(parts[1])
		if tagName == "" {
			continue
		}
		// Only migrate notes that are trips: contain "trip" or match "City Month Year" pattern
		if !isTripTag(tagName) {
			continue
		}
		// Normalise: "Nagpur Apr 2025" -> "nagpur-apr-2025"
		tagName = strings.ToLower(strings.ReplaceAll(tagName, " ", "-"))

		cacheKey := t.BudgetID.String() + ":" + tagName
		tagId, exists := tagCache[cacheKey]
		if !exists {
			// Try to find existing tag or create new one
			err := pool.QueryRow(ctx, `
				INSERT INTO tags (name, budget_id, color, deleted, created_at, updated_at)
				VALUES ($1, $2, '', FALSE, NOW(), NOW())
				ON CONFLICT (name, budget_id) DO UPDATE SET name = EXCLUDED.name
				RETURNING id
			`, tagName, t.BudgetID).Scan(&tagId)
			if err != nil {
				log.Printf("Warning: error creating tag %q for budget %v: %v", tagName, t.BudgetID, err)
				continue
			}
			tagCache[cacheKey] = tagId
			log.Printf("Created/found tag: %q (id: %v) for budget %v", tagName, tagId, t.BudgetID)
		}

		// Add tag to transaction's tag_ids array
		_, err := pool.Exec(ctx, `
			UPDATE transactions SET tag_ids = array_append(tag_ids, $1), updated_at = NOW()
			WHERE id = $2 AND NOT ($1 = ANY(tag_ids))
		`, tagId, t.ID)
		if err != nil {
			log.Printf("Warning: error adding tag %v to transaction %v: %v", tagId, t.ID, err)
			continue
		}

		// Update note to remove the tag prefix
		_, err = pool.Exec(ctx, `
			UPDATE transactions SET note = $1, updated_at = NOW()
			WHERE id = $2
		`, remainingNote, t.ID)
		if err != nil {
			log.Printf("Warning: error updating note for transaction %v: %v", t.ID, err)
		}
	}

	log.Printf("Note-to-tag migration complete. Processed %d transactions.", len(txnNotes))
	return nil
}

// --- Migration orchestration ---

func run(dbConn *pgxpool.Pool, targetName string) {
	ctx := context.Background()

	if err := createSchema(ctx, dbConn); err != nil {
		log.Fatalf("Schema creation failed: %v", err)
	}

	migrations := []struct {
		Version  int
		Name     string
		Path     string
		Migrator Migrator
	}{
		{1, "budgets", "data/budgets.json", genericMigrator[model.Budget]{TableName: "budgets", InsertFn: insertBudgets}},
		{2, "accounts", "data/accounts.json", genericMigrator[model.Account]{TableName: "accounts", InsertFn: insertAccounts}},
		{3, "users", "data/users.json", genericMigrator[model.User]{TableName: "users", InsertFn: insertUsers}},
		{4, "category_groups", "data/categoryGroups.json", genericMigrator[model.CategoryGroup]{TableName: "category_groups", InsertFn: insertCategoryGroups}},
		{5, "categories", "data/categories.json", genericMigrator[model.Category]{TableName: "categories", InsertFn: insertCategories}},
		{6, "inflow_category", "data/inflowCategory.json", genericMigrator[InflowCategory]{TableName: "categories", InsertFn: insertInflowCategory}},
		{7, "payees", "data/payees.json", genericMigrator[model.Payee]{TableName: "payees", InsertFn: insertPayees}},
		{8, "transactions", "data/transactions.json", genericMigrator[model.Transaction]{TableName: "transactions", InsertFn: insertTransactions}},
		{9, "add_tag_ids_to_transactions", "", genericMigrator[struct{}]{TableName: "transactions", InsertFn: addTagIdsColumn}},
		{10, "migrate_notes_to_tags", "", genericMigrator[struct{}]{TableName: "tags", InsertFn: migrateNotesToTags}},
		{11, "create_transaction_embeddings", "", genericMigrator[struct{}]{TableName: "transaction_embeddings", InsertFn: createTransactionEmbeddingsTable}},
	}

	// If a specific migration is targeted, find and run only that one
	if targetName != "" {
		for _, m := range migrations {
			if m.Name == targetName {
				log.Printf("Running targeted migration: %s (v%d)", m.Name, m.Version)
				if err := m.Migrator.Run(ctx, dbConn, m.Path); err != nil {
					log.Fatalf("Error running migration v%d (%s): %v", m.Version, m.Name, err)
				}
				if err := recordMigration(ctx, dbConn, m.Version, m.Name); err != nil {
					log.Fatalf("Error recording migration v%d (%s): %v", m.Version, m.Name, err)
				}
				log.Printf("Applied migration v%d (%s)", m.Version, m.Name)
				return
			}
		}
		log.Fatalf("Unknown migration name: %q. Available: budgets, accounts, users, category_groups, categories, inflow_category, payees, transactions, add_tag_ids_to_transactions, migrate_notes_to_tags, create_transaction_embeddings", targetName)
	}

	// Run all pending migrations in order
	for _, m := range migrations {
		applied, err := isMigrationApplied(ctx, dbConn, m.Version)
		if err != nil {
			log.Fatalf("Failed to check migration status for v%d (%s): %v", m.Version, m.Name, err)
		}
		if applied {
			log.Printf("Skipping migration v%d (%s): already applied", m.Version, m.Name)
			continue
		}

		if err := m.Migrator.Run(ctx, dbConn, m.Path); err != nil {
			log.Fatalf("Error running migration v%d (%s): %v", m.Version, m.Name, err)
		}

		if err := recordMigration(ctx, dbConn, m.Version, m.Name); err != nil {
			log.Fatalf("Error recording migration v%d (%s): %v", m.Version, m.Name, err)
		}
		log.Printf("Applied migration v%d (%s)", m.Version, m.Name)
	}

	log.Printf("All migrations completed")
}

// --- Utility functions for carryover balance computation ---

func createMonthlyBudgetsBackupTable(pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS monthly_budgets_backup AS TABLE monthly_budgets")
	if err != nil {
		log.Printf("error while creating monthly_budgets_backup table: %v", err)
		return
	}
}

func getBalances(pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(
		ctx,
		`
		WITH activity_per_month AS (
				SELECT
					t.budget_id,
					t.category_id,
					LEFT(t.date, 7) AS month,
					SUM(t.amount) AS activity
			FROM transactions t
			GROUP BY t.budget_id, t.category_id, LEFT(t.date, 7)
		),
		mb_ordered AS (
		  SELECT
		    mb.id,
		    mb.carryover_balance,
		    mb.budgeted,
		    COALESCE(apm.activity, 0) AS activity,
		    LAG(mb.carryover_balance) OVER (
		      PARTITION BY mb.budget_id, mb.category_id ORDER BY mb.month
		    ) AS prev_carryover
		  FROM monthly_budgets_backup mb
		  LEFT JOIN activity_per_month apm
		    ON mb.budget_id = apm.budget_id
		    AND mb.category_id = apm.category_id
		    AND mb.month = apm.month
		)
		UPDATE monthly_budgets_backup mb
		SET carryover_balance = COALESCE(mb2.prev_carryover, 0) + mb2.budgeted - mb2.activity
		FROM mb_ordered mb2
		WHERE mb.id = mb2.id
		`,
	)
	if err != nil {
		log.Printf("error while updating monthly_budgets: %v", err)
		return
	}
}

func updateCarryover(pool *pgxpool.Pool) {
	ctx := context.Background()

	// 1. Find all (budget_id, category_id) in monthly_budgets
	rows, err := pool.Query(
		ctx, `
			SELECT DISTINCT budget_id, category_id FROM monthly_budgets
		`,
	)
	if err != nil {
		log.Printf("error while updating carryover: %v", err)
		return
	}
	defer rows.Close()

	var uniqueMonthCat []model.MonthlyBudget
	for rows.Next() {
		var monthlyBudget model.MonthlyBudget
		err := rows.Scan(&monthlyBudget.BudgetID, &monthlyBudget.CategoryID)
		if err != nil {
			log.Printf("error while fetching monthly budgets: %v", err)
			return
		}
		uniqueMonthCat = append(uniqueMonthCat, monthlyBudget)
	}

	// 2. for each monthlyBudget, update carryover_balance for each month
	for _, monthCat := range uniqueMonthCat {
		monthRows, err := pool.Query(
			ctx, `
				SELECT id, month, budget_id, category_id, budgeted, carryover_balance, created_at, updated_at
				FROM monthly_budgets
				WHERE budget_id = $1 AND category_id = $2
				ORDER BY month ASC
			`, monthCat.BudgetID, monthCat.CategoryID,
		)
		if err != nil {
			log.Printf("error while fetching monthly budgets: %v", err)
			return
		}
		defer monthRows.Close()
		var mbRows []model.MonthlyBudget
		for monthRows.Next() {
			var r model.MonthlyBudget
			err := monthRows.Scan(
				&r.ID,
				&r.Month,
				&r.BudgetID,
				&r.CategoryID,
				&r.Budgeted,
				&r.CarryoverBalance,
				&r.CreatedAt,
				&r.UpdatedAt,
			)
			if err != nil {
				log.Printf("error while scanning monthly budgets: %v", err)
				return
			}
			mbRows = append(mbRows, r)
		}

		prevCarryover := 0.0
		// sum transactions for each monthlyBudget month
		for _, b := range mbRows {
			var activity float64

			err = pool.QueryRow(
				ctx, `
					SELECT COALESCE(SUM(amount), 0)
					FROM transactions
					WHERE budget_id = $1 AND category_id = $2 AND date LIKE $3
				`, b.BudgetID, b.CategoryID, b.Month+"%",
			).Scan(&activity)
			if err != nil {
				log.Printf("error while scanning transactions for activity: %v", err)
				return
			}
			carryover := (prevCarryover + b.Budgeted) + activity // adding activity because amount is negative for debit
			rounded_carryover, err := strconv.ParseFloat(fmt.Sprintf("%.2f", carryover), 64)
			if err != nil {
				log.Printf("error while rounding carryover: %v", err)
				return
			}
			_, err = pool.Exec(
				ctx, `
					UPDATE monthly_budgets
					SET carryover_balance=$1
				  WHERE id=$2
				`, rounded_carryover, b.ID,
			)
			if err != nil {
				log.Printf("error while updating carryover balance for id %v: %v", b.ID, err)
				return
			}
			prevCarryover = carryover
		}
	}
	log.Printf("carryover_balance updated for all monthly_budgets")
}

func main() {
	migrateName := flag.String("migrate", "", "Run a specific migration by name (e.g. budgets, accounts, users, categories). If empty, runs all pending migrations.")
	flag.Parse()

	dbConn := db.Connect()
	defer dbConn.Close()

	run(dbConn, *migrateName)
}
