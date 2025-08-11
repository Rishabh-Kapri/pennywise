package repository

import (
	"context"
	"errors"
	"log"
	"time"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PredictionRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Prediction, error)
	Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error)
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, prediction model.Prediction) error
	// DeleteById deletes a prediction by budget and prediction id.
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
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
		`SELECT 
				id,
				budget_id,
				transaction_id,
				email_text,
				amount,
				account,
				account_prediction,
				payee,
				payee_prediction,
				category,
				category_prediction,
				has_user_corrected,
				user_corrected_account,
				user_corrected_payee,
				user_corrected_category,
				created_at,
				updated_at
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
		err := rows.Scan(
			&p.ID,
			&p.BudgetID,
			&p.TransactionID,
			&p.EmailText,
			&p.Amount,
			&p.Account,
			&p.AccountPrediction,
			&p.Payee,
			&p.PayeePrediction,
			&p.Category,
			&p.CategoryPrediction,
			&p.HasUserCorrected,
			&p.UserCorrectedAccount,
			&p.UserCorrectedPayee,
			&p.UserCorrectedCategory,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		predictions = append(predictions, p)
	}
	return predictions, nil
}

// Create inserts a new Prediction record into the predictions table.
// It sets prediction fields, initializes has_user_corrected to false, and user-corrected fields to nil.
// Returns any error encountered during the database operation.
func (r *predictionRepo) Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error) {
	var createdPrediction model.Prediction
	err := r.db.QueryRow(
		ctx,
		`INSERT INTO predictions (
				budget_id,
				transaction_id,
				email_text,
				amount,
				account,
				account_prediction,
				payee,
				payee_prediction,
				category,
				category_prediction,
				has_user_corrected,
				user_corrected_account,
				user_corrected_payee,
				user_corrected_category,
				created_at,
				updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, budget_id, transaction_id, email_text, amount, account, account_prediction, payee, payee_prediction, category, category_prediction, has_user_corrected, created_at`,
		prediction.BudgetID,
		prediction.TransactionID,
		prediction.EmailText,
		prediction.Amount,
		prediction.Account,
		prediction.AccountPrediction,
		prediction.Payee,
		prediction.PayeePrediction,
		prediction.Category,
		prediction.CategoryPrediction,
		false,
		nil,
		nil,
		nil,
		time.Now(),
		time.Now(),
	).Scan(
		&createdPrediction.ID,
		&createdPrediction.BudgetID,
		&createdPrediction.TransactionID,
		&createdPrediction.EmailText,
		&createdPrediction.Amount,
		&createdPrediction.Account,
		&createdPrediction.AccountPrediction,
		&createdPrediction.Payee,
		&createdPrediction.PayeePrediction,
		&createdPrediction.Category,
		&createdPrediction.CategoryPrediction,
		&createdPrediction.HasUserCorrected,
		&createdPrediction.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	predictions := make([]model.Prediction, 1)
	return append(predictions, createdPrediction), nil
}

// Update modifies an existing Prediction record in the predictions table.
// It updates all fields except for created_at, setting updated_at to the current time.
// Returns any error encountered during the database operation.
func (r *predictionRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, prediction model.Prediction) error {
	cmdTag, err := r.db.Exec(
		ctx,
		`UPDATE predictions SET
				transaction_id = $1,
				email_text = $2,
				amount = $3,
				account = $4,
				account_prediction = $5,
				payee = $6,
				payee_prediction = $7,
				category = $8,
				category_prediction = $9,
				has_user_corrected = $10,
				user_corrected_account = $11,
				user_corrected_payee = $12,
				user_corrected_category = $13,
				updated_at = NOW()
		WHERE budget_id = $14 AND id = $15`,
		prediction.TransactionID,
		prediction.EmailText,
		prediction.Amount,
		prediction.Account,
		prediction.AccountPrediction,
		prediction.Payee,
		prediction.PayeePrediction,
		prediction.Category,
		prediction.CategoryPrediction,
		prediction.HasUserCorrected,
		prediction.UserCorrectedAccount,
		prediction.UserCorrectedPayee,
		prediction.UserCorrectedCategory,
		budgetId,
		id,
	)
	log.Printf("%v, %v", cmdTag, err)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("Provide a valid id")
	}

	return nil
}

// DeleteById marks the prediction entry as deleted (soft delete) with the specified budgetId and id.
func (r *predictionRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	cmdTag, err := r.db.Exec(
		ctx,
		`UPDATE predictions SET deleted = TRUE WHERE budget_id = $1 AND id = $2 AND deleted = FALSE`,
		budgetId,
		id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("No active prediction found with the given id and budgetId")
	}
	return nil
}
