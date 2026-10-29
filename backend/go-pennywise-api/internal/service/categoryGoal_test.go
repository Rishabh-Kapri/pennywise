package service

import (
	"context"
	"testing"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockCategoryGoalRepo struct {
	repository.BaseRepository
	mock.Mock
}

func (m *mockCategoryGoalRepo) Upsert(ctx context.Context, budgetId, categoryId uuid.UUID, goal model.CategoryGoal) error {
	return m.Called(ctx, budgetId, categoryId, goal).Error(0)
}

func (m *mockCategoryGoalRepo) Delete(ctx context.Context, budgetId, categoryId uuid.UUID) error {
	return m.Called(ctx, budgetId, categoryId).Error(0)
}

func newTestGoalService(repo *mockCategoryGoalRepo) *categoryGoalService {
	return &categoryGoalService{
		repo: repo,
		now:  func() time.Time { return time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC) },
	}
}

func TestCategoryGoalService_Upsert(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("normalizes_before_storing", func(t *testing.T) {
		repo := &mockCategoryGoalRepo{}
		repo.On("Upsert", mock.Anything, budgetID, catID, mock.MatchedBy(func(g model.CategoryGoal) bool {
			return g.Cadence == model.GoalCadenceMonthly && g.CadenceFrequency == 1 && g.CreationMonth == "2026-10"
		})).Return(nil)
		goal, err := newTestGoalService(repo).
			Upsert(ctx, catID, model.CategoryGoal{Type: model.GoalTypeNeed, Target: 300})
		assert.NoError(t, err)
		assert.Equal(t, model.GoalCadenceMonthly, goal.Cadence)
		repo.AssertExpectations(t)
	})

	t.Run("invalid_goal_never_reaches_the_repo", func(t *testing.T) {
		repo := &mockCategoryGoalRepo{}
		_, err := newTestGoalService(repo).
			Upsert(ctx, catID, model.CategoryGoal{Type: model.GoalTypeTargetBalanceByDate, Target: 300})
		assert.ErrorIs(t, err, model.ErrInvalidGoal)
		repo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("missing_category_is_not_found", func(t *testing.T) {
		repo := &mockCategoryGoalRepo{}
		repo.On("Upsert", mock.Anything, budgetID, catID, mock.Anything).Return(pgx.ErrNoRows)
		_, err := newTestGoalService(repo).
			Upsert(ctx, catID, model.CategoryGoal{Type: model.GoalTypeMonthlyFunding, Target: 50})
		assert.ErrorIs(t, err, ErrCategoryNotFound)
	})
}

func TestCategoryGoalService_Delete(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("deletes", func(t *testing.T) {
		repo := &mockCategoryGoalRepo{}
		repo.On("Delete", mock.Anything, budgetID, catID).Return(nil)
		assert.NoError(t, newTestGoalService(repo).Delete(ctx, catID))
		repo.AssertExpectations(t)
	})

	t.Run("no_goal_is_not_found", func(t *testing.T) {
		repo := &mockCategoryGoalRepo{}
		repo.On("Delete", mock.Anything, budgetID, catID).Return(pgx.ErrNoRows)
		assert.ErrorIs(t, newTestGoalService(repo).Delete(ctx, catID), ErrCategoryGoalNotFound)
	})
}
