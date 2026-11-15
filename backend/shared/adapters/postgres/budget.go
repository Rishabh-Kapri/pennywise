package postgres

import (
	"context"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type budgetRepo struct {
	BaseRepository
}

func NewBudgetRepository(pool *pgxpool.Pool) repository.BudgetRepository {
	return &budgetRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *budgetRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
			SELECT b.id, b.user_id, b.name, m.is_selected, b.created_at, b.updated_at,
			       COALESCE(b.metadata, '{}'), b.onboarding, m.role,
			       (SELECT count(*) FROM budget_members c WHERE c.budget_id = b.id)
			FROM budget_members m
			JOIN budgets b ON b.id = m.budget_id
			WHERE m.user_id = $1 AND b.deleted = FALSE
			ORDER BY b.created_at
		`, userID,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var budgets []model.Budget

	for rows.Next() {
		var b model.Budget
		err := rows.Scan(
			&b.ID, &b.UserID, &b.Name, &b.IsSelected, &b.CreatedAt, &b.UpdatedAt,
			&b.Metadata, &b.Onboarding, &b.Role, &b.MemberCount,
		)
		if err != nil {
			return nil, err
		}
		budgets = append(budgets, b)
	}
	return budgets, rows.Err()
}

// GetById reads the budget alone; IsSelected and Role belong to a member and
// are left empty.
func (r *budgetRepo) GetById(ctx context.Context, id uuid.UUID) (*model.Budget, error) {
	var budget model.Budget
	err := r.Executor(ctx).QueryRow(
		ctx, `
				SELECT id, user_id, name, created_at, updated_at, COALESCE(metadata, '{}'), onboarding
				FROM budgets
				WHERE id = $1 AND deleted = FALSE
			`, id,
	).Scan(&budget.ID, &budget.UserID, &budget.Name, &budget.CreatedAt, &budget.UpdatedAt, &budget.Metadata, &budget.Onboarding)
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

func (r *budgetRepo) Create(ctx context.Context, name string, userID uuid.UUID) (*model.Budget, error) {
	createdBudget := model.Budget{UserID: userID, Role: model.BudgetRoleOwner, MemberCount: 1}
	err := r.Executor(ctx).QueryRow(
		ctx, `
			WITH b AS (
				INSERT INTO budgets (name, user_id, is_selected, created_at, updated_at)
				VALUES ($1, $2, FALSE, NOW(), NOW())
				RETURNING id, name, created_at, updated_at
			), m AS (
				-- A user's first budget (none selected yet) opens selected.
				INSERT INTO budget_members (budget_id, user_id, role, is_selected)
				SELECT id, $2, 'owner',
				       NOT EXISTS (SELECT 1 FROM budget_members WHERE user_id = $2 AND is_selected)
				FROM b
				RETURNING is_selected
			)
			SELECT b.id, b.name, b.created_at, b.updated_at, m.is_selected FROM b, m
			`, name, userID,
	).Scan(&createdBudget.ID, &createdBudget.Name, &createdBudget.CreatedAt, &createdBudget.UpdatedAt, &createdBudget.IsSelected)
	if err != nil {
		return nil, err
	}
	return &createdBudget, nil
}

func (r *budgetRepo) UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error {
	cmdTag, err := r.Executor(ctx).Exec(
		ctx, `
			UPDATE budgets SET name = $1, metadata = $2, updated_at = NOW()
			WHERE id = $3 AND deleted = FALSE
			`, budget.Name, budget.Metadata, id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("budget not found for id %v", id)
	}
	return nil
}

func (r *budgetRepo) Rename(ctx context.Context, id uuid.UUID, name string) error {
	cmdTag, err := r.Executor(ctx).Exec(
		ctx,
		`UPDATE budgets SET name = $1, updated_at = NOW() WHERE id = $2 AND deleted = FALSE`,
		name, id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *budgetRepo) UpdateOnboarding(ctx context.Context, id uuid.UUID, onboarding model.BudgetOnboarding) error {
	cmdTag, err := r.Executor(ctx).Exec(
		ctx,
		`UPDATE budgets SET onboarding = $1, updated_at = NOW() WHERE id = $2 AND deleted = FALSE`,
		onboarding, id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("budget not found for id %v", id)
	}
	return nil
}
