package db

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CipherPredictionRepository interface {
	BaseRepositoryInterface
	Create(ctx context.Context, tx pgx.Tx, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error)
	GetAll(ctx context.Context, budgetID uuid.UUID) ([]model.CipherPredictionRecord, error)
	GetByTransactionID(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) (*model.CipherPredictionRecord, error)
	MarkUserCorrected(ctx context.Context, tx pgx.Tx, budgetID uuid.UUID, txnID uuid.UUID, actualPayeeID *uuid.UUID, actualCategoryID *uuid.UUID) error
	MarkLearned(ctx context.Context, tx pgx.Tx, budgetID uuid.UUID, txnID uuid.UUID) error
	MarkLearnFailed(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID, reason string) error
	ListPendingLearning(ctx context.Context, budgetID uuid.UUID, limit int, retryFailed bool) ([]model.LearningCandidate, error)
	LearningStats(ctx context.Context, budgetID uuid.UUID) (model.LearningStats, error)
}

type cipherPredictionRepo struct {
	BaseRepository
}

func NewCipherPredictionRepository(pool *pgxpool.Pool) CipherPredictionRepository {
	return &cipherPredictionRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *cipherPredictionRepo) Create(
	ctx context.Context,
	tx pgx.Tx,
	p model.CipherPredictionRecord,
) (*model.CipherPredictionRecord, error) {
	var created model.CipherPredictionRecord
	now := time.Now()

	err := r.Executor(tx).QueryRow(
		ctx,
		`INSERT INTO cipher_predictions (
			budget_id,
			transaction_id,
			email_text,
			llm_reasoning,
			metadata,
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
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING
			id, budget_id, transaction_id, email_text, llm_reasoning, metadata, amount,
			extracted_account, extracted_payee,
			predicted_payee_id, predicted_category_id,
			account_confidence, payee_confidence, category_confidence,
			source, has_user_corrected,
			actual_payee_id, actual_category_id,
			learned_at, learn_error,
			created_at, updated_at, deleted`,
		p.BudgetID,
		p.TransactionID,
		p.EmailText,
		p.LLMReasoning,
		p.Metadata,
		p.Amount,
		p.ExtractedAccount,
		p.ExtractedPayee,
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
		&created.LLMReasoning,
		&created.Metadata,
		&created.Amount,
		&created.ExtractedAccount,
		&created.ExtractedPayee,
		&created.PredictedPayeeID,
		&created.PredictedCategoryID,
		&created.AccountConfidence,
		&created.PayeeConfidence,
		&created.CategoryConfidence,
		&created.Source,
		&created.HasUserCorrected,
		&created.ActualPayeeID,
		&created.ActualCategoryID,
		&created.LearnedAt,
		&created.LearnError,
		&created.CreatedAt,
		&created.UpdatedAt,
		&created.Deleted,
	)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *cipherPredictionRepo) GetAll(
	ctx context.Context,
	budgetID uuid.UUID,
) ([]model.CipherPredictionRecord, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT
			id, budget_id, transaction_id, email_text, llm_reasoning, metadata, amount,
			extracted_account, extracted_payee,
			predicted_payee_id, predicted_category_id,
			account_confidence, payee_confidence, category_confidence,
			source, has_user_corrected,
			actual_payee_id, actual_category_id,
			learned_at, learn_error,
			created_at, updated_at, deleted
		FROM cipher_predictions
		WHERE budget_id = $1 AND deleted = FALSE
		ORDER BY created_at DESC`,
		budgetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.CipherPredictionRecord
	for rows.Next() {
		var p model.CipherPredictionRecord
		err := rows.Scan(
			&p.ID,
			&p.BudgetID,
			&p.TransactionID,
			&p.EmailText,
			&p.LLMReasoning,
			&p.Metadata,
			&p.Amount,
			&p.ExtractedAccount,
			&p.ExtractedPayee,
			&p.PredictedPayeeID,
			&p.PredictedCategoryID,
			&p.AccountConfidence,
			&p.PayeeConfidence,
			&p.CategoryConfidence,
			&p.Source,
			&p.HasUserCorrected,
			&p.ActualPayeeID,
			&p.ActualCategoryID,
			&p.LearnedAt,
			&p.LearnError,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.Deleted,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, nil
}

func (r *cipherPredictionRepo) GetByTransactionID(
	ctx context.Context,
	budgetID uuid.UUID,
	txnID uuid.UUID,
) (*model.CipherPredictionRecord, error) {
	var p model.CipherPredictionRecord
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT
			id, budget_id, transaction_id, email_text, llm_reasoning, metadata, amount,
			extracted_account, extracted_payee,
			predicted_payee_id, predicted_category_id,
			account_confidence, payee_confidence, category_confidence,
			source, has_user_corrected,
			actual_payee_id, actual_category_id,
			learned_at, learn_error,
			created_at, updated_at, deleted
		FROM cipher_predictions
		WHERE budget_id = $1 AND transaction_id = $2 AND deleted = FALSE`,
		budgetID, txnID,
	).Scan(
		&p.ID,
		&p.BudgetID,
		&p.TransactionID,
		&p.EmailText,
		&p.LLMReasoning,
		&p.Metadata,
		&p.Amount,
		&p.ExtractedAccount,
		&p.ExtractedPayee,
		&p.PredictedPayeeID,
		&p.PredictedCategoryID,
		&p.AccountConfidence,
		&p.PayeeConfidence,
		&p.CategoryConfidence,
		&p.Source,
		&p.HasUserCorrected,
		&p.ActualPayeeID,
		&p.ActualCategoryID,
		&p.LearnedAt,
		&p.LearnError,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.Deleted,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *cipherPredictionRepo) MarkUserCorrected(
	ctx context.Context,
	tx pgx.Tx,
	budgetID uuid.UUID,
	txnID uuid.UUID,
	actualPayeeID *uuid.UUID,
	actualCategoryID *uuid.UUID,
) error {
	cmdTag, err := r.Executor(tx).Exec(
		ctx,
		`UPDATE cipher_predictions
		 SET has_user_corrected = TRUE,
		     actual_payee_id = $1,
		     actual_category_id = $2,
		     updated_at = NOW()
		 WHERE budget_id = $3 AND transaction_id = $4 AND deleted = FALSE`,
		actualPayeeID,
		actualCategoryID,
		budgetID,
		txnID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// MarkLearned records that this prediction has taught the pipeline, clearing any
// error from an earlier failed attempt.
func (r *cipherPredictionRepo) MarkLearned(
	ctx context.Context,
	tx pgx.Tx,
	budgetID uuid.UUID,
	txnID uuid.UUID,
) error {
	cmdTag, err := r.Executor(tx).Exec(
		ctx,
		`UPDATE cipher_predictions
		 SET learned_at = NOW(),
		     learn_error = NULL,
		     updated_at = NOW()
		 WHERE budget_id = $1 AND transaction_id = $2 AND deleted = FALSE`,
		budgetID,
		txnID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// MarkLearnFailed records why learning failed, leaving learned_at NULL so the
// row stays in the backfill's pending set. Runs outside any caller transaction:
// the failure it records is usually the reason that transaction is being rolled
// back.
func (r *cipherPredictionRepo) MarkLearnFailed(
	ctx context.Context,
	budgetID uuid.UUID,
	txnID uuid.UUID,
	reason string,
) error {
	_, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE cipher_predictions
		 SET learn_error = $1,
		     updated_at = NOW()
		 WHERE budget_id = $2 AND transaction_id = $3 AND deleted = FALSE`,
		truncateLearnError(reason),
		budgetID,
		txnID,
	)
	return err
}

// learnErrorMaxLen bounds what is stored: these come from upstream errors that
// can carry a whole response body, and the column is only ever read by a human.
const learnErrorMaxLen = 500

func truncateLearnError(reason string) string {
	if len(reason) <= learnErrorMaxLen {
		return reason
	}
	return reason[:learnErrorMaxLen]
}

// ListPendingLearning returns transactions whose prediction has never been
// learned from, oldest first so a repeated backfill walks forward through
// history instead of re-reading the same page.
//
// Only rows the learning step can actually use are returned: it needs a payee, a
// category and the raw bank text future emails are matched against.
func (r *cipherPredictionRepo) ListPendingLearning(
	ctx context.Context,
	budgetID uuid.UUID,
	limit int,
	retryFailed bool,
) ([]model.LearningCandidate, error) {
	if limit <= 0 {
		limit = 1
	}
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT t.id, t.payee_id, t.category_id, t.amount, t.raw_bank_text, cp.learn_error
		 FROM cipher_predictions cp
		 JOIN transactions t ON t.id = cp.transaction_id
		 WHERE cp.budget_id = $1
		   AND cp.deleted = FALSE
		   AND cp.learned_at IS NULL
		   AND ($2 OR cp.learn_error IS NULL)
		   AND t.deleted = FALSE
		   AND t.payee_id IS NOT NULL
		   AND t.category_id IS NOT NULL
		   AND COALESCE(BTRIM(t.raw_bank_text), '') != ''
		 ORDER BY cp.created_at ASC
		 LIMIT $3`,
		budgetID, retryFailed, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []model.LearningCandidate
	for rows.Next() {
		var c model.LearningCandidate
		if err := rows.Scan(
			&c.TransactionID,
			&c.PayeeID,
			&c.CategoryID,
			&c.Amount,
			&c.RawBankText,
			&c.LearnError,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

// LearningStats counts how much of the budget's prediction history has taught
// the pipeline. Pending counts only rows a backfill could actually process --
// it applies the same eligibility filter as ListPendingLearning, so a caller can
// loop until Pending reaches zero. Rows that can never be learned from (no
// payee, no category, no raw bank text, deleted transaction) are counted as
// Skipped instead, and Total = Learned + Pending + Skipped.
func (r *cipherPredictionRepo) LearningStats(
	ctx context.Context,
	budgetID uuid.UUID,
) (model.LearningStats, error) {
	var stats model.LearningStats
	err := r.Executor(nil).QueryRow(
		ctx,
		`WITH scoped AS (
			SELECT
				cp.learned_at,
				cp.learn_error,
				(
					t.id IS NOT NULL
					AND t.deleted = FALSE
					AND t.payee_id IS NOT NULL
					AND t.category_id IS NOT NULL
					AND COALESCE(BTRIM(t.raw_bank_text), '') != ''
				) AS learnable
			FROM cipher_predictions cp
			LEFT JOIN transactions t ON t.id = cp.transaction_id
			WHERE cp.budget_id = $1 AND cp.deleted = FALSE
		)
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE learned_at IS NOT NULL),
			COUNT(*) FILTER (WHERE learned_at IS NULL AND learnable),
			COUNT(*) FILTER (WHERE learned_at IS NULL AND learnable AND learn_error IS NOT NULL),
			COUNT(*) FILTER (WHERE learned_at IS NULL AND NOT learnable)
		FROM scoped`,
		budgetID,
	).Scan(&stats.Total, &stats.Learned, &stats.Pending, &stats.Failed, &stats.Skipped)
	if err != nil {
		return model.LearningStats{}, err
	}
	return stats, nil
}
