package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upSeedPayeesTxns, downSeedPayeesTxns)
}

func upSeedPayeesTxns(ctx context.Context, tx *sql.Tx) error {
	// Payees
	if payees, err := loadFileData[model.Payee]("data/payees.json"); err == nil {
		for _, d := range payees {
			_, err = tx.ExecContext(ctx, `INSERT INTO payees (id, name, budget_id, transfer_account_id, deleted, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING`,
				d.ID, d.Name, d.BudgetID, uuidOrNull(d.TransferAccountID), d.Deleted, d.CreatedAt, d.UpdatedAt)
			if err != nil {
				return err
			}
		}
	} else {
		log.Printf("Skipping payees seed: %v", err)
	}

	// Transactions
	if transactions, err := loadFileData[model.Transaction]("data/transactions.json"); err == nil {
		for _, d := range transactions {
			_, err = tx.ExecContext(ctx, `INSERT INTO transactions (id, budget_id, date, payee_id, category_id, account_id, note, amount, source, transfer_account_id, transfer_transaction_id, deleted, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ON CONFLICT DO NOTHING`,
				d.ID, d.BudgetID, d.Date, uuidOrNull(d.PayeeID), uuidOrNull(d.CategoryID), uuidOrNull(d.AccountID), d.Note, d.Amount, d.Source, uuidOrNull(d.TransferAccountID), uuidOrNull(d.TransferTransactionID), d.Deleted, d.CreatedAt, d.UpdatedAt)
			if err != nil {
				return err
			}
		}
	} else {
		log.Printf("Skipping transactions seed: %v", err)
	}

	return nil
}

func downSeedPayeesTxns(ctx context.Context, tx *sql.Tx) error {
	return nil
}
