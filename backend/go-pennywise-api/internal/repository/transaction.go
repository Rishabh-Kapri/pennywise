package repository

import (
	"context"
	"errors"
	"log"

	"pennywise-api/internal/model"

	utils "pennywise-api/pkg"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Transaction, error)
	GetAllNormalized(ctx context.Context, budgetId uuid.UUID) ([]model.Transaction, error)
	// GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, txn model.Transaction) error
	Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error)
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}

type transactionRepo struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) TransactionRepository {
	return &transactionRepo{db: db}
}

func (r *transactionRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Transaction, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, budget_id, date, payee_id, category_id, account_id, note, amount, transfer_account_id, transfer_transaction_id, created_at, updated_at
		 FROM transactions 
		 WHERE budget_id = $1 AND deleted = FALSE
		 ORDER BY date DESC, updated_at DESC;`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []model.Transaction
	for rows.Next() {
		var txn model.Transaction
		err := rows.Scan(&txn.ID, &txn.BudgetID, &txn.Date, &txn.PayeeID, &txn.CategoryID, &txn.AccountID, &txn.Note, &txn.Amount, &txn.TransferAccountID, &txn.TransferTransactionID, &txn.CreatedAt, &txn.UpdatedAt)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}
	return transactions, nil
}

func (r *transactionRepo) GetAllNormalized(ctx context.Context, budgetId uuid.UUID) ([]model.Transaction, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT
				transactions.id, transactions.budget_id, transactions.date, transactions.payee_id, transactions.category_id, transactions.account_id, transactions.note, transactions.amount, transactions.transfer_account_id, transactions.transfer_transaction_id, transactions.created_at, transactions.updated_at,
				accounts.name AS account_name,
				payees.name AS payee_name,
				categories.name AS category_name
			FROM
				transactions
			LEFT JOIN accounts   ON transactions.account_id  = accounts.id
			LEFT JOIN payees     ON transactions.payee_id    = payees.id
			LEFT JOIN categories ON transactions.category_id = categories.id
			WHERE transactions.budget_id = $1 AND transactions.deleted = FALSE
			ORDER BY date DESC, updated_at DESC;`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []model.Transaction
	for rows.Next() {
		var txn model.Transaction
		err := rows.Scan(
			&txn.ID,
			&txn.BudgetID,
			&txn.Date,
			&txn.PayeeID,
			&txn.CategoryID,
			&txn.AccountID,
			&txn.Note,
			&txn.Amount,
			&txn.TransferAccountID,
			&txn.TransferTransactionID,
			&txn.CreatedAt,
			&txn.UpdatedAt,
			&txn.AccountName,
			&txn.PayeeName,
			&txn.CategoryName,
		)
		if err != nil {
			return nil, err
		}
		txns = append(txns, txn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return txns, nil
}

func (r *transactionRepo) Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()
	var createdTxn model.Transaction
	err = tx.QueryRow(
		ctx,
		`INSERT INTO transactions (
			budget_id,
		  date,
		  payee_id,
		  category_id,
		  account_id,
		  note,
		  source,
		  amount,
		  transfer_account_id,
		  transfer_transaction_id,
		  created_at,
		  updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id, amount, budget_id`,
		txn.BudgetID,
		txn.Date,
		txn.PayeeID,
		txn.CategoryID,
		txn.AccountID,
		txn.Note,
		txn.Source,
		txn.Amount,
		txn.TransferAccountID,
		txn.TransferTransactionID,
	).Scan(&createdTxn.ID, &createdTxn.Amount, &createdTxn.BudgetID)
	if err != nil {
		return nil, err
	}
	// only update when categoryId is present
	// TODO: move the Inflow category check to utils
	if txn.CategoryID != nil && txn.CategoryID.String() != "02fc5abc-94b7-4b03-9077-5d153011fd3f" {
		monthKey := utils.GetMonthKey(txn.Date)
		if err := utils.UpdateCarryover(ctx, tx, txn.BudgetID, *txn.CategoryID, txn.Amount, monthKey); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	txns := make([]model.Transaction, 0)
	return append(txns, createdTxn), nil
}

func (r *transactionRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, txn model.Transaction) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	cmdTag, err := tx.Exec(
		ctx,
		`UPDATE transactions SET 
				date = $1,
				payee_id = $2,
				category_id = $3,
				account_id = $4,
				note = $5,
				amount = $6, 
				transfer_account_id = $7,
				transfer_transaction_id = $8,
				updated_at = NOW()
		 WHERE budget_id = $9 AND id = $10`,
		txn.Date,
		txn.PayeeID,
		txn.CategoryID,
		txn.AccountID,
		txn.Note,
		txn.Amount,
		txn.TransferAccountID,
		txn.TransferTransactionID,
		budgetId,
		id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("Provide a valid id")
	}
	if txn.CategoryID != nil {
		monthKey := utils.GetMonthKey(txn.Date)
		if err = utils.UpdateCarryover(ctx, tx, budgetId, *txn.CategoryID, txn.Amount, monthKey); err != nil {
			return err
		}
	}

	return nil
}

func (r *transactionRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	var updatedTxn model.Transaction
	err = tx.QueryRow(
		ctx,
		`UPDATE transactions 
		SET deleted = TRUE WHERE budget_id = $1 AND id = $2 
		RETURNING id, budget_id, date, payee_id, category_id, account_id, amount`,
		budgetId, id,
	).Scan(&updatedTxn.ID, &updatedTxn.BudgetID, &updatedTxn.Date, &updatedTxn.PayeeID, &updatedTxn.CategoryID, &updatedTxn.AccountID, &updatedTxn.Amount)
	if err != nil {
		return err
	}
	if updatedTxn.ID == uuid.Nil {
		return errors.New("Provide a valid id")
	}

	// Reverse the amount for updation
	monthKey := utils.GetMonthKey(updatedTxn.Date)
	if err = utils.UpdateCarryover(ctx, tx, budgetId, *updatedTxn.CategoryID, -(updatedTxn.Amount), monthKey); err != nil {
		return err
	}
	log.Printf("Soft deleted transaction with id: %v", id)
	return nil
}
