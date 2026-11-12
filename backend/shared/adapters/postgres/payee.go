package postgres

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type payeeRepo struct {
	BaseRepository
}

func NewPayeesRepository(pool *pgxpool.Pool) repository.PayeesRepository {
	return &payeeRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *payeeRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Payee, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
		SELECT id, name, budget_id, transfer_account_id, created_at, updated_at
		FROM payees WHERE budget_id = $1 AND deleted = FALSE`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payees []model.Payee
	for rows.Next() {
		var payee model.Payee
		err := rows.Scan(&payee.ID, &payee.Name, &payee.BudgetID, &payee.TransferAccountID, &payee.CreatedAt, &payee.UpdatedAt)
		if err != nil {
			return nil, err
		}
		payees = append(payees, payee)
	}
	return payees, nil
}

func (r *payeeRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Payee, error) {
	rows, err := r.Executor(ctx).Query(
		ctx,
		`SELECT id, name, budget_id, transfer_account_id, created_at, updated_at FROM payees 
		   WHERE budget_id = $1 AND deleted = FALSE AND name = $2`,
		// budgetId, "%"+query+"%",
		budgetId, query,
		// budgetId, "[[:<:]]"+query+"[[:>:]]",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var payees []model.Payee
	for rows.Next() {
		var payee model.Payee
		err := rows.Scan(&payee.ID, &payee.Name, &payee.BudgetID, &payee.TransferAccountID, &payee.CreatedAt, &payee.UpdatedAt)
		if err != nil {
			return nil, err
		}
		payees = append(payees, payee)
	}
	return payees, nil
}

func (r *payeeRepo) GetById(ctx context.Context, budgetId, id uuid.UUID) (*model.Payee, error) {
	var payee model.Payee
	err := r.Executor(ctx).QueryRow(
		ctx, `
		  SELECT id, name, budget_id, transfer_account_id
		  FROM payees
		  WHERE id = $1 AND budget_id = $2 AND deleted = FALSE
		`, id, budgetId,
	).Scan(
		&payee.ID,
		&payee.Name,
		&payee.BudgetID,
		&payee.TransferAccountID,
	)
	if err != nil {
		return nil, err
	}
	return &payee, nil
}

func (r *payeeRepo) Create(ctx context.Context, payee model.Payee) (*model.Payee, error) {
	var createdPayee model.Payee

	err := r.Executor(ctx).QueryRow(
		ctx, `
				INSERT INTO payees (
				name, budget_id, transfer_account_id
				) VALUES ($1, $2, $3)
				RETURNING id, name, transfer_account_id
			`, payee.Name, payee.BudgetID, payee.TransferAccountID,
	).Scan(&createdPayee.ID, &createdPayee.Name, &createdPayee.TransferAccountID)
	if err != nil {
		return nil, err
	}
	return &createdPayee, nil
}

func (r *payeeRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	// Retire the payee's rules with it: cipher resolves a matched rule's payee
	// with deleted = FALSE, so a surviving rule would fail the email pipeline.
	cmdTag, err := r.Executor(ctx).Exec(
		ctx,
		`WITH retired_rules AS (
		   UPDATE payee_rules SET deleted = TRUE, updated_at = NOW()
		   WHERE payee_id = $1 AND budget_id = $2 AND deleted = FALSE
		 )
		 UPDATE payees SET
		   deleted = TRUE,
		   updated_at = NOW()
		WHERE id = $1 AND budget_id = $2`,
		id, budgetId,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("Payee not found")
	}
	return nil
}

func (r *payeeRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payee model.Payee) error {
	cmdTag, err := r.Executor(ctx).Exec(
		ctx,
		`UPDATE payees SET
		   name = $1,
			 updated_at = NOW()
		WHERE id = $2 AND budget_id = $3`,
		payee.Name, id, budgetId,
	)

	if cmdTag.RowsAffected() == 0 {
		return errors.New("Payee not found")
	}

	return err
}

func (r *payeeRepo) GetStats(ctx context.Context, budgetId uuid.UUID) ([]model.PayeeStats, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
		SELECT p.id,
		       COALESCE(t.txn_count, 0),
		       COALESCE(t.inflow, 0)::float8,
		       COALESCE(t.outflow, 0)::float8,
		       t.last_used,
		       t.top_category_id,
		       COALESCE(r.rule_count, 0)
		FROM payees p
		LEFT JOIN (
		  SELECT payee_id,
		         COUNT(*) AS txn_count,
		         SUM(amount) FILTER (WHERE amount > 0) AS inflow,
		         -SUM(amount) FILTER (WHERE amount < 0) AS outflow,
		         MAX(date) AS last_used,
		         mode() WITHIN GROUP (ORDER BY category_id) AS top_category_id
		  FROM transactions
		  WHERE budget_id = $1 AND deleted = FALSE AND payee_id IS NOT NULL
		  GROUP BY payee_id
		) t ON t.payee_id = p.id
		LEFT JOIN (
		  SELECT payee_id, COUNT(*) AS rule_count
		  FROM payee_rules
		  WHERE budget_id = $1 AND deleted = FALSE
		  GROUP BY payee_id
		) r ON r.payee_id = p.id
		WHERE p.budget_id = $1 AND p.deleted = FALSE`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []model.PayeeStats{}
	for rows.Next() {
		var st model.PayeeStats
		if err := rows.Scan(
			&st.PayeeID,
			&st.TransactionCount,
			&st.TotalInflow,
			&st.TotalOutflow,
			&st.LastUsed,
			&st.TopCategoryID,
			&st.RuleCount,
		); err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}
	return stats, rows.Err()
}

// Merge moves everything that references the source payees onto the target,
// then soft-deletes the sources, all in one transaction (the caller's, when
// ctx carries one). The caller validates the ids.
func (r *payeeRepo) Merge(
	ctx context.Context,
	budgetId, targetId uuid.UUID,
	sourceIds []uuid.UUID,
) (*model.PayeeMergeResult, error) {
	var result *model.PayeeMergeResult
	err := r.InTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = r.merge(ctx, budgetId, targetId, sourceIds)
		return err
	})
	return result, err
}

func (r *payeeRepo) merge(
	ctx context.Context,
	budgetId, targetId uuid.UUID,
	sourceIds []uuid.UUID,
) (*model.PayeeMergeResult, error) {
	result := &model.PayeeMergeResult{TargetID: targetId, MergedPayees: len(sourceIds)}

	repoint := func(query string) (int64, error) {
		tag, err := r.Executor(ctx).Exec(ctx, query, targetId, sourceIds, budgetId)
		if err != nil {
			return 0, err
		}
		return tag.RowsAffected(), nil
	}

	var err error
	if result.Transactions, err = repoint(`
		UPDATE transactions SET payee_id = $1, updated_at = NOW()
		WHERE payee_id = ANY($2) AND budget_id = $3`); err != nil {
		return nil, err
	}
	if result.Rules, err = repoint(`
		UPDATE payee_rules SET payee_id = $1, updated_at = NOW()
		WHERE payee_id = ANY($2) AND budget_id = $3 AND deleted = FALSE`); err != nil {
		return nil, err
	}
	if result.Recurring, err = repoint(`
		UPDATE recurring_transactions SET payee_id = $1, updated_at = NOW()
		WHERE payee_id = ANY($2) AND budget_id = $3`); err != nil {
		return nil, err
	}
	if result.Embeddings, err = repoint(`
		UPDATE transaction_embeddings SET payee_id = $1, updated_at = NOW()
		WHERE payee_id = ANY($2) AND budget_id = $3`); err != nil {
		return nil, err
	}
	if _, err = repoint(`
		UPDATE cipher_predictions SET
		  predicted_payee_id = CASE WHEN predicted_payee_id = ANY($2) THEN $1 ELSE predicted_payee_id END,
		  actual_payee_id = CASE WHEN actual_payee_id = ANY($2) THEN $1 ELSE actual_payee_id END,
		  updated_at = NOW()
		WHERE (predicted_payee_id = ANY($2) OR actual_payee_id = ANY($2)) AND budget_id = $3`); err != nil {
		return nil, err
	}

	// the sources' name embeddings would keep resolving queries to dead ids
	if _, err = r.Executor(ctx).Exec(ctx, `
		DELETE FROM entity_embeddings
		WHERE entity_type = 'payee' AND entity_id = ANY($1) AND budget_id = $2`,
		sourceIds, budgetId,
	); err != nil {
		return nil, err
	}

	if _, err = r.Executor(ctx).Exec(ctx, `
		UPDATE payees SET deleted = TRUE, updated_at = NOW()
		WHERE id = ANY($1) AND budget_id = $2`,
		sourceIds, budgetId,
	); err != nil {
		return nil, err
	}
	return result, nil
}
