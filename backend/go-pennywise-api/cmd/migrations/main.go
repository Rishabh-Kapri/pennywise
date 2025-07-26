package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"pennywise-api/internal/db"
	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type migration[T any] struct {
	data []T
}

type InflowCategory struct {
	model.Category
}

type genericMigrator[T any] struct {
	TableName string
	InsertFn  func(ctx context.Context, db *pgxpool.Pool, data []T) error
}

// const BUDGET_ID = "7974d2a6-688f-11f0-8536-9c6b002128a7"

func loadFileData[T any](path string) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Error while opening %v: %v", path, err.Error())
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Error while reading file %v: %v", path, err.Error())
		return nil, err
	}
	var data []T
	if err := json.Unmarshal(bytes, &data); err != nil {
		log.Printf("Error while unmarshaling file data: %v %v", path, err.Error())
		return nil, err
	}
	log.Printf("Loaded total data for file %v: %v\n", path, len(data))

	return data, nil
}

func runMigration[T any](ctx context.Context, db *pgxpool.Pool, m genericMigrator[T], path string) error {
	var data []T
	if path != "" {
		data1, err := loadFileData[T](path)
		if err != nil {
			return fmt.Errorf("runMigration: error while loading file %v", err.Error())
		}
		data = data1
	}
	if err := m.InsertFn(ctx, db, data); err != nil {
		return fmt.Errorf("runMigration: error while inserting data for: %v, %v", path, err.Error())
	}
	log.Printf("Migrated %s: %d items", m.TableName, len(data))
	return nil
}

func insertBudgets(ctx context.Context, db *pgxpool.Pool, data []model.Budget) error {
	createBudgetTableQuery := `
		CREATE TABLE IF NOT EXISTS budgets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			is_selected BOOLEAN,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`
	_, err := db.Exec(ctx, createBudgetTableQuery)
	if err != nil {
		log.Fatalf("Error while creating accounts table %v", err.Error())
	}
	batch := &pgx.Batch{}
	for _, d := range data {
		batch.Queue(
			`INSERT INTO budgets (
				id, name, is_selected, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
			d.ID, d.Name, d.IsSelected, d.CreatedAt, d.UpdatedAt,
		)
	}
	br := db.SendBatch(ctx, batch)
	defer br.Close()
	return br.Close()
}

func alterAccounts(ctx context.Context, db *pgxpool.Pool) error {
	alterAccountTableQuery := `
		ALTER TABLE accounts
    ADD CONSTRAINT fk_transfer_payee_id
		FOREIGN KEY (transfer_payee_id) REFERENCES payees(id)
	`
	_, err := db.Exec(ctx, alterAccountTableQuery)
	if err != nil {
		log.Fatalf("Error while altering accounts table %v", err.Error())
	}
	return err
}

func alterTransactions(ctx context.Context, db *pgxpool.Pool) error {
	alterTxnTableQuery := `
		ALTER TABLE transactions
    ADD CONSTRAINT fk_transfer_transaction_id
		FOREIGN KEY (transfer_transaction_id) REFERENCES transactions(id)
	`
	_, err := db.Exec(ctx, alterTxnTableQuery)
	if err != nil {
		log.Fatalf("Error while altering accounts table %v", err.Error())
	}
	return err
}

func insertAccounts(ctx context.Context, db *pgxpool.Pool, data []model.Account) error {
	createAccountTableQuery := `
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
		)`
	createPayeeTableQuery := `
		CREATE TABLE IF NOT EXISTS payees (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			budget_id UUID NOT NULL REFERENCES budgets(id),
			transfer_account_id UUID REFERENCES accounts(id),
			deleted BOOLEAN DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`
	_, err := db.Exec(ctx, createAccountTableQuery)
	if err != nil {
		log.Fatalf("Error while creating accounts table %v", err.Error())
	}
	_, err = db.Exec(ctx, createPayeeTableQuery)
	if err != nil {
		log.Fatalf("Error while creating payees table %v", err.Error())
	}
	batch := &pgx.Batch{}
	for _, d := range data {
		id, _ := uuid.Parse(d.ID)
		var transferPayeeId uuid.UUID
		if d.TransferPayeeID != "" {
			transferPayeeId, _ = uuid.Parse(d.TransferPayeeID)
		}
		batch.Queue(
			`INSERT INTO accounts (
				id, name, budget_id, transfer_payee_id, type, closed, deleted, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9
			) ON CONFLICT DO NOTHING`,
			id, d.Name, d.BudgetID, transferPayeeId, d.Type, d.Closed, d.Deleted, d.CreatedAt, d.UpdatedAt,
		)
	}
	br := db.SendBatch(ctx, batch)
	defer br.Close()
	return br.Close()
}

func insertCategoryGroups(ctx context.Context, db *pgxpool.Pool, data []model.CategoryGroup) error {
	createCategoryGroupTableQuery := `
	CREATE TABLE IF NOT EXISTS category_groups (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name TEXT NOT NULL,
		budget_id UUID NOT NULL REFERENCES budgets(id),
		hidden BOOLEAN DEFAULT false,
		is_system BOOLEAN DEFAULT false,
		deleted BOOLEAN DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`
	_, err := db.Exec(ctx, createCategoryGroupTableQuery)
	if err != nil {
		log.Fatalf("Error while creating category_groups table %v", err.Error())
	}
	batch := &pgx.Batch{}
	for _, d := range data {
		isSystem := false
		if d.Name == "Credit Card Payments" {
			isSystem = true
		}
		var id uuid.UUID
		if err := uuid.Validate(d.ID); err != nil {
			id, _ = uuid.NewUUID()
		} else {
			id, _ = uuid.Parse(d.ID)
		}
		batch.Queue(
			`INSERT INTO category_groups (
				id, name, budget_id, hidden, is_system, deleted, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT DO NOTHING`,
			id, d.Name, d.BudgetID, d.Hidden, isSystem, d.Deleted, d.CreatedAt, d.UpdatedAt,
		)
	}
	br := db.SendBatch(ctx, batch)
	defer br.Close()
	return br.Close()
}

func insertMonthlyBudgets(ctx context.Context, db *pgxpool.Pool, data map[string]float32, budgetId uuid.UUID, categoryId uuid.UUID) error {
	// log.Printf("Inserting monthly budgets for category %v", categoryId)
	createMonthlyBudgetsTableQuery := `
	CREATE TABLE IF NOT EXISTS monthly_budgets (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		month TEXT NOT NULL,
		budget_id UUID NOT NULL REFERENCES budgets(id),
		category_id UUID NOT NULL REFERENCES categories(id),
		budgeted REAL NOT NULL,
		carryover_balance REAL NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`
	_, err := db.Exec(ctx, createMonthlyBudgetsTableQuery)
	if err != nil {
		log.Fatalf("Error while creating monthly_budgets table %v", err.Error())
	}
	batch := &pgx.Batch{}
	for month, budgeted := range data {
		key := strings.Split(month, "-")
		keyFloat, err := strconv.ParseFloat(key[1], 10)
		if err != nil {
			log.Fatalf("Cannot convert to float %v", err.Error())
		}
		keyInt := int(keyFloat) + 1
		newKey := key[0] + "-" + fmt.Sprintf("%02d", keyInt)
		batch.Queue(
			`INSERT INTO monthly_budgets (
				month, budget_id, category_id, budgeted, carryover_balance
			) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
			newKey, budgetId, categoryId, budgeted, 0,
		)
	}
	br := db.SendBatch(ctx, batch)
	defer br.Close()
	return br.Close()
}

func convertFileDataToUUID() {
	file, _ := os.Open("data/payees.json")
	defer file.Close()

	bytes, _ := io.ReadAll(file)

	var data []model.Payee

	_ = json.Unmarshal(bytes, &data)

	dataJson, err := json.Marshal(data)
	// for i := range data {
	// 	d := &data[i]
	// id, _ := uuid.Parse(d.ID)
	// if err := uuid.Validate(d.ID); err != nil {
	// 	log.Printf("Error while validating uuid %v :%v", d.ID, err.Error())
	// }
	// d.Uuid, _ = uuid.Parse(d.ID)
	// log.Printf("%v %v %v", id, id.String(), d.Uuid)
	// }
	if err != nil {
		log.Fatalf("Error while writing file %v", err.Error())
	}
	err = os.WriteFile("data/payees.json", dataJson, 0o644)
}

func insertCategories(ctx context.Context, db *pgxpool.Pool, data []model.Category) error {
	createAccountTableQuery := `
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
	)`
	_, err := db.Exec(ctx, createAccountTableQuery)
	if err != nil {
		log.Fatalf("Error while creating categories table %v", err.Error())
	}
	batch := &pgx.Batch{}
	for _, d := range data {
		isSystem := false
		if d.Name == "Inflow: Ready to Assign" {
			isSystem = true
		}
		id, _ := uuid.Parse(d.ID)
		budgetId, _ := uuid.Parse(d.BudgetID)
		categoryGroupId, _ := uuid.Parse(d.CategoryGroupID)
		if !isSystem {
			// @INFO: Uncomment when running migration
			insertMonthlyBudgets(ctx, db, d.Budgeted, budgetId, id)
		}
		batch.Queue(
			`INSERT INTO categories (
				id, name, budget_id, category_group_id, note, hidden, is_system, deleted, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
			id, d.Name, budgetId, categoryGroupId, d.Note, d.Hidden, isSystem, d.Deleted, d.CreatedAt, d.UpdatedAt,
		)
	}
	br := db.SendBatch(ctx, batch)
	defer br.Close()
	return br.Close()
}

func insertInflowCategory(ctx context.Context, db *pgxpool.Pool, data []InflowCategory) error {
	inflowCat := data[0]
	_, err := db.Exec(ctx, `
		INSERT INTO categories (
			id, name, budget_id, category_group_id, note, hidden, is_system, deleted, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
		inflowCat.ID, inflowCat.Name, inflowCat.BudgetID, inflowCat.CategoryGroupID, inflowCat.Note, false, true, false, inflowCat.CreatedAt, inflowCat.UpdatedAt,
	)
	return err
}

func insertPayees(ctx context.Context, db *pgxpool.Pool, data []model.Payee) error {
	batch := &pgx.Batch{}
	for _, d := range data {
		// id, _ := uuid.Parse(d.ID)
		// _, err := db.Exec(ctx, `
		// 	INSERT INTO payees (
		// 		id, name, budget_id, transfer_account_id, deleted, created_at, updated_at
		// 	) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING
		// 	`,
		// 	id, d.Name, d.BudgetID, d.TransferAccountID, d.Deleted, d.CreatedAt, d.UpdatedAt,
		// )
		// if err != nil {
		// 	log.Printf("Error while insert payee: %v %v", d.ID, err.Error())
		// 	return err
		// }
		var transferAccId *uuid.UUID
		if d.TransferAccountID != "" {
			parsedId, _ := uuid.Parse(d.TransferAccountID)
			transferAccId = &parsedId
		} else {
			transferAccId = nil
		}
		// log.Printf("id: %v, %v transferAccId: %v", id, d.TransferAccountID, transferAccId)
		batch.Queue(`
			INSERT INTO payees (
				id, name, budget_id, transfer_account_id, deleted, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING
			`,
			d.ID, d.Name, d.BudgetID, transferAccId, d.Deleted, d.CreatedAt, d.UpdatedAt,
		)
	}
	br := db.SendBatch(ctx, batch)
	// alterAccounts(ctx, db)
	defer br.Close()
	return br.Close()
}

func insertTransactions(ctx context.Context, db *pgxpool.Pool, data []model.Transaction) error {
	createTransactionTableQuery := `
	CREATE TABLE IF NOT EXISTS transactions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		budget_id UUID NOT NULL REFERENCES budgets(id),
		date TEXT NOT NULL,
		payee_id UUID REFERENCES payees(id),
		category_id UUID REFERENCES categories(id),
		account_id UUID NOT NULL REFERENCES accounts(id),
		note TEXT,
		amount REAL NOT NULL,
		source TEXT,
		transfer_account_id UUID REFERENCES accounts(id),
		transfer_transaction_id UUID, 
		deleted BOOLEAN DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`
	_, err := db.Exec(ctx, createTransactionTableQuery)
	if err != nil {
		log.Fatalf("Error while creating transactions table %v", err.Error())
	}
	batch := &pgx.Batch{}
	for _, d := range data {
		var accountId *uuid.UUID
		if d.AccountID != "" {
			parsedId, _ := uuid.Parse(d.AccountID)
			accountId = &parsedId
		} else {
			accountId = nil
		}
		var payeeId *uuid.UUID
		if d.PayeeID != "" {
			parsedId, _ := uuid.Parse(d.PayeeID)
			payeeId = &parsedId
		} else {
			payeeId = nil
		}
		var catId *uuid.UUID
		if d.CategoryID != "" {
			parsedId, _ := uuid.Parse(d.CategoryID)
			catId = &parsedId
		} else {
			catId = nil
		}
		var transferAccId *uuid.UUID
		if d.TransferAccountID != "" {
			parsedId, _ := uuid.Parse(d.TransferAccountID)
			transferAccId = &parsedId
		} else {
			transferAccId = nil
		}
		var transferTxnId *uuid.UUID
		if d.TransferTransactionID != "" {
			parsedId, _ := uuid.Parse(d.TransferTransactionID)
			transferTxnId = &parsedId
		} else {
			transferTxnId = nil
		}
		batch.Queue(
			`INSERT INTO transactions (
				id, budget_id, date, payee_id, category_id, account_id, note, amount, source, transfer_account_id, transfer_transaction_id, deleted, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
			) ON CONFLICT DO NOTHING`,
			d.ID, d.BudgetID, d.Date, payeeId, catId, accountId, d.Note, d.Amount, d.Source, transferAccId, transferTxnId, d.Deleted, d.CreatedAt, d.UpdatedAt,
		)
		// _, err = db.Exec(ctx, `
		// INSERT INTO transactions (
		// 		id, budget_id, date, payee_id, category_id, account_id, note, amount, source, transfer_account_id, transfer_transaction_id, deleted, created_at, updated_at
		// 	) VALUES (
		// 		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		// 	) ON CONFLICT DO NOTHING
		// 	`,
		// 	d.ID, d.BudgetID, d.Date, payeeId, catId, accountId, d.Note, d.Amount, d.Source, transferAccId, transferTxnId, d.Deleted, d.CreatedAt, d.UpdatedAt,
		// )
		// if err != nil {
		// 	log.Printf("Error while insert transactions: %v %v %v %v %v %v %v", d.ID, accountId, payeeId, catId, transferAccId, transferTxnId, err.Error())
		// 	return err
		// }
	}
	br := db.SendBatch(ctx, batch)
	alterTransactions(ctx, db)
	defer br.Close()
	return br.Close()
}

func insertPredictions(ctx context.Context, db *pgxpool.Pool, data []model.Prediction) error {
	createPredictionTableQuery := `
	CREATE TABLE IF NOT EXISTS predictions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email_text TEXT,
		account TEXT,
		account_prediction REAL,
		payee TEXT,
		payee_prediction REAL,
		category TEXT,
		category_prediction REAL,
		has_user_corrected BOOLEAN,
		user_corrected_account TEXT,
		user_corrected_payee TEXT,
		user_corrected_category TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	)
	`
	_, err := db.Exec(ctx, createPredictionTableQuery)
	if err != nil {
		log.Fatalf("Error while creating predictions table %v", err.Error())
	}
	return err
}

func main() {
	dbConn := db.Connect()

	defer dbConn.Close()

	// convertFileDataToUUID()

	migrations := []struct {
		Path     string
		Migrator any
	}{
		// {"data/budgets.json", genericMigrator[model.Budget]{TableName: "budgets", InsertFn: insertBudgets}},
		{"data/accounts.json", genericMigrator[model.Account]{TableName: "accounts", InsertFn: insertAccounts}},
		{"data/categoryGroups.json", genericMigrator[model.CategoryGroup]{TableName: "category_groups", InsertFn: insertCategoryGroups}},
		{"data/categories.json", genericMigrator[model.Category]{TableName: "categories", InsertFn: insertCategories}},
		{"data/inflowCategory.json", genericMigrator[InflowCategory]{TableName: "categories", InsertFn: insertInflowCategory}},
		{"data/payees.json", genericMigrator[model.Payee]{TableName: "payees", InsertFn: insertPayees}},
		{"data/transactions.json", genericMigrator[model.Transaction]{TableName: "transactions", InsertFn: insertTransactions}},
		{"", genericMigrator[model.Prediction]{TableName: "predictions", InsertFn: insertPredictions}},
	}

	ctx := context.Background()
	// "njBUP4D7VhvDxQq1TRtB"

	for _, migration := range migrations {
		switch m := migration.Migrator.(type) {
		case genericMigrator[model.Budget]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running budgets migration: %v", err)
			}
		case genericMigrator[model.Account]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running accounts migration: %v", err)
			}
		case genericMigrator[model.CategoryGroup]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running categoryGroups migrations: %v", err)
			}
		case genericMigrator[model.Category]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running categories migrations: %v", err)
			}
		case genericMigrator[InflowCategory]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running inflow category migrations: %v", err)
			}
		case genericMigrator[model.Payee]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running payees migration :%v", err)
			}
		case genericMigrator[model.Transaction]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running transactions migration: %v", err)
			}
		}
	}
}
