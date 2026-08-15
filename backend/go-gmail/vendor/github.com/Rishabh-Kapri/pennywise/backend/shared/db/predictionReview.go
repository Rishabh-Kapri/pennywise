package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PredictionReviewRepository serves the cipher-prediction review queue. It is
// separate from CipherPredictionRepository so the ingestion path's interface
// stays focused on writing predictions.
type PredictionReviewRepository interface {
	BaseRepositoryInterface
	GetQueue(ctx context.Context, budgetID uuid.UUID, includeReviewed bool, limit int) ([]model.PredictionReviewItem, error)
	GetByID(ctx context.Context, budgetID uuid.UUID, id uuid.UUID) (*model.PredictionReviewItem, error)
	// MarkReviewed stamps reviewed_at. When actual ids are supplied (the
	// accept path) they are recorded as the confirmed labels.
	MarkReviewed(ctx context.Context, budgetID uuid.UUID, id uuid.UUID, actualPayeeID, actualCategoryID *uuid.UUID) error
}

type predictionReviewRepo struct {
	BaseRepository
}

func NewPredictionReviewRepository(pool *pgxpool.Pool) PredictionReviewRepository {
	return &predictionReviewRepo{BaseRepository: NewBaseRepository(pool)}
}

const predictionReviewSelect = `
	SELECT
		cp.id, cp.budget_id, cp.transaction_id, cp.email_text, cp.llm_reasoning, cp.metadata, cp.amount,
		cp.extracted_account, cp.extracted_payee,
		cp.predicted_payee_id, cp.predicted_category_id,
		cp.account_confidence, cp.payee_confidence, cp.category_confidence,
		cp.source, cp.has_user_corrected,
		cp.actual_payee_id, cp.actual_category_id, cp.reviewed_at,
		cp.created_at, cp.updated_at, cp.deleted,
		t.date, t.amount, COALESCE(t.note, ''),
		a.name,
		t.payee_id, cur_p.name,
		t.category_id, cur_c.name,
		pred_p.name, pred_c.name
	FROM cipher_predictions cp
	JOIN transactions t ON t.id = cp.transaction_id AND t.deleted = FALSE
	LEFT JOIN accounts a ON a.id = t.account_id
	LEFT JOIN payees cur_p ON cur_p.id = t.payee_id
	LEFT JOIN categories cur_c ON cur_c.id = t.category_id
	LEFT JOIN payees pred_p ON pred_p.id = cp.predicted_payee_id
	LEFT JOIN categories pred_c ON pred_c.id = cp.predicted_category_id
`

func scanPredictionReviewItem(rows pgx.Rows) (model.PredictionReviewItem, error) {
	var item model.PredictionReviewItem
	err := rows.Scan(
		&item.ID,
		&item.BudgetID,
		&item.TransactionID,
		&item.EmailText,
		&item.LLMReasoning,
		&item.Metadata,
		&item.Amount,
		&item.ExtractedAccount,
		&item.ExtractedPayee,
		&item.PredictedPayeeID,
		&item.PredictedCategoryID,
		&item.AccountConfidence,
		&item.PayeeConfidence,
		&item.CategoryConfidence,
		&item.Source,
		&item.HasUserCorrected,
		&item.ActualPayeeID,
		&item.ActualCategoryID,
		&item.ReviewedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Deleted,
		&item.TransactionDate,
		&item.TransactionAmount,
		&item.TransactionNote,
		&item.AccountName,
		&item.CurrentPayeeID,
		&item.CurrentPayeeName,
		&item.CurrentCategoryID,
		&item.CurrentCategoryName,
		&item.PredictedPayeeName,
		&item.PredictedCategoryName,
	)
	return item, err
}

func (r *predictionReviewRepo) GetQueue(
	ctx context.Context,
	budgetID uuid.UUID,
	includeReviewed bool,
	limit int,
) ([]model.PredictionReviewItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := predictionReviewSelect + ` WHERE cp.budget_id = $1 AND cp.deleted = FALSE`
	if !includeReviewed {
		query += ` AND cp.reviewed_at IS NULL`
	}
	// least-confident first: those are the ones worth a human's attention
	query += `
		ORDER BY LEAST(
			COALESCE(cp.payee_confidence, 1),
			COALESCE(cp.category_confidence, 1)
		) ASC, cp.created_at DESC
		LIMIT $2`

	rows, err := r.Executor(nil).Query(ctx, query, budgetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.PredictionReviewItem, 0)
	for rows.Next() {
		item, err := scanPredictionReviewItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *predictionReviewRepo) GetByID(
	ctx context.Context,
	budgetID uuid.UUID,
	id uuid.UUID,
) (*model.PredictionReviewItem, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		predictionReviewSelect+` WHERE cp.budget_id = $1 AND cp.id = $2 AND cp.deleted = FALSE`,
		budgetID,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, pgx.ErrNoRows
	}
	item, err := scanPredictionReviewItem(rows)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *predictionReviewRepo) MarkReviewed(
	ctx context.Context,
	budgetID uuid.UUID,
	id uuid.UUID,
	actualPayeeID *uuid.UUID,
	actualCategoryID *uuid.UUID,
) error {
	cmdTag, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE cipher_predictions
		 SET reviewed_at = NOW(),
		     actual_payee_id = COALESCE($1, actual_payee_id),
		     actual_category_id = COALESCE($2, actual_category_id),
		     updated_at = NOW()
		 WHERE budget_id = $3 AND id = $4 AND deleted = FALSE`,
		actualPayeeID,
		actualCategoryID,
		budgetID,
		id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
