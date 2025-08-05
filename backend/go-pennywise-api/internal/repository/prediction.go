package repository

import (
	"context"
	"time"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PredictionRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Prediction, error)
	Create(ctx context.Context, prediction model.Prediction) error
}

type predictionRepo struct {
	db *pgxpool.Pool
}

func NewPredictionRepository(db *pgxpool.Pool) PredictionRepository {
	return &predictionRepo{db: db}
}

// @TODO: add support for search query
func (r *predictionRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Prediction, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, budget_id, transaction_id, email_text, amount, account, account_prediction, payee, payee_prediction, category, category_prediction, has_user_corrected, user_corrected_account, user_corrected_payee, user_corrected_category, created_at, updated_at
		 FROM predictions 
		 WHERE budget_id = $1;`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var predictions []model.Prediction
	for rows.Next() {
		var p model.Prediction
		err := rows.Scan(&p.ID, &p.BudgetID, &p.TransactionID, &p.EmailText, &p.Amount, &p.Account, &p.AccountPrediction, &p.Payee, &p.PayeePrediction, &p.Category, &p.CategoryPrediction, &p.HasUserCorrected, &p.UserCorrectedAccount, &p.UserCorrectedPayee, &p.UserCorrectedCategory, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		predictions = append(predictions, p)
	}
	return predictions, nil
}

func (r *predictionRepo) Create(ctx context.Context, prediction model.Prediction) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO predictions (
			budget_id, transaction_id, email_text, amount, account, account_prediction, payee, payee_prediction, category, category_prediction, has_user_corrected, user_corrected_account, user_corrected_payee, user_corrected_category, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		prediction.BudgetID, prediction.TransactionID, prediction.EmailText, prediction.Amount, prediction.Account, prediction.AccountPrediction, prediction.Payee, prediction.PayeePrediction, prediction.Category, prediction.CategoryPrediction, false, nil, nil, nil, time.Now(), time.Now(),
	)
	return err
}
