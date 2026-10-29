package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CategoryGoalRepository stores category targets (category_goals), at most one
// per category.
type CategoryGoalRepository interface {
	BaseRepositoryInterface
	// Upsert creates or replaces the category's target. It returns
	// pgx.ErrNoRows when the category is not in the budget, is deleted, or is
	// the inflow (system) category, which cannot carry a target.
	Upsert(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, goal model.CategoryGoal) error
	// Delete removes the category's target; pgx.ErrNoRows when it has none.
	Delete(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID) error
}

type categoryGoalRepo struct {
	BaseRepository
}

func NewCategoryGoalRepository(pool *pgxpool.Pool) CategoryGoalRepository {
	return &categoryGoalRepo{BaseRepository: NewBaseRepository(pool)}
}

// categoryGoalJSON is a scalar subquery rendering the target of the category
// whose id is categoryIDExpr as a JSON object shaped like model.CategoryGoal,
// or NULL when it has none. Category reads embed it so a category arrives with
// its target in one round-trip.
func categoryGoalJSON(categoryIDExpr string) string {
	return `(
		SELECT json_build_object(
			'type', g.type,
			'target', g.target,
			'cadence', g.cadence,
			'cadenceFrequency', g.cadence_frequency,
			'day', g.day,
			'needsWholeAmount', g.needs_whole_amount,
			'targetMonth', g.target_month,
			'creationMonth', g.creation_month,
			'snoozedMonth', g.snoozed_month
		)
		FROM category_goals g
		WHERE g.category_id = ` + categoryIDExpr + `
	)`
}

func nullIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullIfZero(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func (r *categoryGoalRepo) Upsert(
	ctx context.Context,
	budgetId uuid.UUID,
	categoryId uuid.UUID,
	goal model.CategoryGoal,
) error {
	// Selecting from categories both checks the category belongs to the budget
	// and supplies budget_id, so a goal can never point across budgets.
	cmdTag, err := r.Executor(nil).Exec(
		ctx, `
			INSERT INTO category_goals (
				budget_id, category_id, type, target, cadence, cadence_frequency, day,
				needs_whole_amount, target_month, creation_month, snoozed_month
			)
			SELECT
				c.budget_id, c.id, $3::text, $4::numeric, $5::text, $6::integer, $7::smallint,
				$8::boolean, $9::text, $10::text, $11::text
			FROM categories c
			WHERE c.budget_id = $1 AND c.id = $2 AND c.deleted = FALSE AND c.is_system = FALSE
			ON CONFLICT (category_id) DO UPDATE SET
				type = EXCLUDED.type,
				target = EXCLUDED.target,
				cadence = EXCLUDED.cadence,
				cadence_frequency = EXCLUDED.cadence_frequency,
				day = EXCLUDED.day,
				needs_whole_amount = EXCLUDED.needs_whole_amount,
				target_month = EXCLUDED.target_month,
				creation_month = EXCLUDED.creation_month,
				snoozed_month = EXCLUDED.snoozed_month,
				updated_at = NOW()
		`,
		budgetId,
		categoryId,
		string(goal.Type),
		goal.Target,
		nullIfEmpty(string(goal.Cadence)),
		nullIfZero(goal.CadenceFrequency),
		goal.Day,
		goal.NeedsWholeAmount,
		nullIfEmpty(goal.TargetMonth),
		goal.CreationMonth,
		nullIfEmpty(goal.SnoozedMonth),
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *categoryGoalRepo) Delete(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID) error {
	cmdTag, err := r.Executor(nil).Exec(
		ctx,
		`DELETE FROM category_goals WHERE budget_id = $1 AND category_id = $2`,
		budgetId,
		categoryId,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
