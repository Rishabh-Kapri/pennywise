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

func uuidOrNull(id *uuid.UUID) interface{} {
	if id != nil && *id == uuid.Nil {
		return nil
	}
	return id
}

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
		returnedData, err := loadFileData[T](path)
		if err != nil {
			return fmt.Errorf("runMigration: error while loading file %v", err.Error())
		}
		data = returnedData
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

func alterAccountsTable(ctx context.Context, db *pgxpool.Pool) error {
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

func alterTransactionsTable(ctx context.Context, db *pgxpool.Pool) error {
	alterTxnTableQuery := `
		ALTER TABLE transactions
			ADD CONSTRAINT fk_transfer_transaction_id FOREIGN KEY (transfer_transaction_id) REFERENCES transactions(id),
			ADD CONSTRAINT fk_payee_id FOREIGN KEY (payee_id) REFERENCES payees(id),
			ADD CONSTRAINT fk_account_id FOREIGN KEY (account_id) REFERENCES accounts(id),
			ADD CONSTRAINT fk_category_id FOREIGN KEY (category_id) REFERENCES categories(id),
			ADD CONSTRAINT fk_transfer_account_id FOREIGN KEY (transfer_account_id) REFERENCES accounts(id);
	`
	_, err := db.Exec(ctx, alterTxnTableQuery)
	if err != nil {
		log.Fatalf("Error while altering transactions table %v", err.Error())
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
	br := db.SendBatch(ctx, batch)
	defer br.Close()
	return br.Close()
}

func insertUsers(ctx context.Context, db *pgxpool.Pool, data []model.User) error {
	createUsersTableQuery := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		budget_id UUID NOT NULL REFERENCES budgets(id),
		email TEXT NOT NULL,
	  history_id NUMERIC(10, 0) NOT NULL,
		deleted BOOLEAN DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`
	_, err := db.Exec(ctx, createUsersTableQuery)
	if err != nil {
		log.Fatalf("Error while creating users table %v", err.Error())
	}
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
		budgeted NUMERIC(12, 2) NOT NULL,
		carryover_balance NUMERIC(12, 2) NOT NULL,
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
		batch.Queue(`
				INSERT INTO monthly_budgets (
					budget_id, category_id, month, budgeted, carryover_balance, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
				ON CONFLICT DO NOTHING
			`, budgetId, categoryId, newKey, budgeted, 0,
		)
	}
	br := db.SendBatch(ctx, batch)
	defer br.Close()
	return br.Close()
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
		if !isSystem {
			// @INFO: Uncomment when running migration
			insertMonthlyBudgets(ctx, db, d.Budgeted, d.BudgetID, d.ID)
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
	br := db.SendBatch(ctx, batch)
	// alterAccountsTable(ctx, db)
	defer br.Close()
	return br.Close()
}

func insertTransactions(ctx context.Context, db *pgxpool.Pool, data []model.Transaction) error {
	createTransactionTableQuery := `
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
	)`
	_, err := db.Exec(ctx, createTransactionTableQuery)
	if err != nil {
		log.Fatalf("Error while creating transactions table %v", err.Error())
	}
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

		// _, err = db.Exec(ctx, `
		//
		// INSERT INTO transactions (
		//
		// id, budget_id, date, payee_id, category_id, account_id, note, amount, source, transfer_account_id, transfer_transaction_id, deleted, created_at, updated_at
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
	alterTransactionsTable(ctx, db)
	defer br.Close()
	return br.Close()
}

func insertPredictions(ctx context.Context, db *pgxpool.Pool, data []model.Prediction) error {
	createPredictionTableQuery := `
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
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)
	`
	_, err := db.Exec(ctx, createPredictionTableQuery)
	if err != nil {
		log.Fatalf("Error while creating predictions table %v", err.Error())
	}
	log.Printf("Predictions table created")
	return err
}

func run(dbConn *pgxpool.Pool) {
	migrations := []struct {
		Path     string
		Migrator any
	}{
		{"data/budgets.json", genericMigrator[model.Budget]{TableName: "budgets", InsertFn: insertBudgets}},
		{"data/accounts.json", genericMigrator[model.Account]{TableName: "accounts", InsertFn: insertAccounts}},
		{"data/users.json", genericMigrator[model.User]{TableName: "users", InsertFn: insertUsers}},
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
		case genericMigrator[model.User]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running users migrations: %v", err)
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
		case genericMigrator[model.Prediction]:
			if err := runMigration(ctx, dbConn, m, migration.Path); err != nil {
				log.Printf("Error while running predictions migration: %v", err)
			}
		}
	}
}

func createMonthlyBudgetsBackupTable(db *pgxpool.Pool) {
	ctx := context.Background()
	_, err := db.Exec(ctx, "CREATE TABLE IF NOT EXISTS monthly_budgets_backup AS TABLE monthly_budgets")
	if err != nil {
		log.Printf("error while creating monthly_budgets_backup table: %v", err)
		return
	}
}

func getBalances(db *pgxpool.Pool) {
	ctx := context.Background()
	_, err := db.Exec(
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

func updateCarryover(db *pgxpool.Pool) {
	ctx := context.Background()

	// 1. Find all (budget_id, category_id) in monthly_budgets
	rows, err := db.Query(
		ctx, `
			SELECT DISTINCT budget_id, category_id FROM monthly_budgets
		`,
	)
	if err != nil {
		log.Printf("error while updating carryover: %v", err)
		return
	}
	defer rows.Close()

	var unqiueMonthCat []model.MonthlyBudget
	for rows.Next() {
		var monthlyBudget model.MonthlyBudget
		err := rows.Scan(&monthlyBudget.BudgetID, &monthlyBudget.CategoryID)
		if err != nil {
			log.Printf("error while fetching monthly budgets: %v", err)
			return
		}
		unqiueMonthCat = append(unqiueMonthCat, monthlyBudget)
	}

	// 2. for each monthlyBudget, update carryover_balance for each month
	for _, monthCat := range unqiueMonthCat {
		monthRows, err := db.Query(
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
			// log.Printf("%+v", b)
			var activity float64

			err = db.QueryRow(
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
			_, err = db.Exec(
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
	dbConn := db.Connect()

	defer dbConn.Close()

	// run(dbConn)
	// createMonthlyBudgetsBackupTable(dbConn)
	updateCarryover(dbConn)
}
