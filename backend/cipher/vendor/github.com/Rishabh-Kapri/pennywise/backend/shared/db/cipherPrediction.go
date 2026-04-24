package db

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CipherPredictionRepository interface {
	BaseRepositoryInterface
	Create(ctx context.Context, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error)
	GetByTransactionID(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) (*model.CipherPredictionRecord, error)
}

type cipherPredictionRepo struct {
	BaseRepository
}

func NewCipherPredictionRepository(pool *pgxpool.Pool) CipherPredictionRepository {
	return &cipherPredictionRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *cipherPredictionRepo) Create(ctx context.Context, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
	var created model.CipherPredictionRecord
	now := time.Now()

	err := r.Executor(nil).QueryRow(
		ctx,
		`INSERT INTO cipher_predictions (
			budget_id,
			transaction_id,
			email_text,
			amount,
			extracted_account,
			extracted_payee,
			predicted_payee_id,
			predicted_category_id,
			account_confidence,
			payee_confidence,
			category_confidence,
			source,
			has_user_corrected,
			actual_payee_id,
			actual_category_id,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING
			id, budget_id, transaction_id, email_text, amount,
			extracted_account, extracted_payee,
			predicted_payee_id, predicted_category_id,
			account_confidence, payee_confidence, category_confidence,
			source, has_user_corrected,
			actual_payee_id, actual_category_id,
			created_at, updated_at, deleted`,
		p.BudgetID,
		p.TransactionID,
		p.EmailText,
		p.Amount,
		p.ExtractedAccount,
		p.ExtractedMerchant,
		p.PredictedPayeeID,
		p.PredictedCategoryID,
		p.AccountConfidence,
		p.PayeeConfidence,
		p.CategoryConfidence,
		p.Source,
		false,
		nil, // actual_payee_id
		nil, // actual_category_id
		now,
		now,
	).Scan(
		&created.ID,
		&created.BudgetID,
		&created.TransactionID,
		&created.EmailText,
		&created.Amount,
		&created.ExtractedAccount,
		&created.ExtractedMerchant,
		&created.PredictedPayeeID,
		&created.PredictedCategoryID,
		&created.AccountConfidence,
		&created.PayeeConfidence,
		&created.CategoryConfidence,
		&created.Source,
		&created.HasUserCorrected,
		&created.ActualPayeeID,
		&created.ActualCategoryID,
		&created.CreatedAt,
		&created.UpdatedAt,
		&created.Deleted,
	)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *cipherPredictionRepo) GetByTransactionID(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) (*model.CipherPredictionRecord, error) {
	var p model.CipherPredictionRecord
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT
			id, budget_id, transaction_id, email_text, amount,
			extracted_account, extracted_payee,
			predicted_payee_id, predicted_category_id,
			account_confidence, payee_confidence, category_confidence,
			source, has_user_corrected,
			actual_payee_id, actual_category_id,
			created_at, updated_at, deleted
		FROM cipher_predictions
		WHERE budget_id = $1 AND transaction_id = $2 AND deleted = FALSE`,
		budgetID, txnID,
	).Scan(
		&p.ID,
		&p.BudgetID,
		&p.TransactionID,
		&p.EmailText,
		&p.Amount,
		&p.ExtractedAccount,
		&p.ExtractedMerchant,
		&p.PredictedPayeeID,
		&p.PredictedCategoryID,
		&p.AccountConfidence,
		&p.PayeeConfidence,
		&p.CategoryConfidence,
		&p.Source,
		&p.HasUserCorrected,
		&p.ActualPayeeID,
		&p.ActualCategoryID,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.Deleted,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
