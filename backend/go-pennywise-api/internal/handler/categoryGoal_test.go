package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockCategoryGoalService struct{ mock.Mock }

func (m *mockCategoryGoalService) Upsert(ctx context.Context, id uuid.UUID, goal model.CategoryGoal) (*model.CategoryGoal, error) {
	args := m.Called(ctx, id, goal)
	g, _ := args.Get(0).(*model.CategoryGoal)
	return g, args.Error(1)
}

func (m *mockCategoryGoalService) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func TestCategoryGoalHandler_Upsert(t *testing.T) {
	id := uuid.New()
	goal := model.CategoryGoal{Type: model.GoalTypeMonthlyFunding, Target: 200}
	run := func(svc *mockCategoryGoalService, body any) int {
		w, c := makeReq("PUT", "/categories/"+id.String()+"/goal", body)
		c.Params = gin.Params{{Key: "id", Value: id.String()}}
		NewCategoryGoalHandler(svc).Upsert(c)
		return w.Code
	}
	t.Run("stores_goal", func(t *testing.T) {
		svc := &mockCategoryGoalService{}
		svc.On("Upsert", mock.Anything, id, goal).Return(&goal, nil)
		assert.Equal(t, http.StatusOK, run(svc, goal))
	})
	t.Run("invalid_goal_returns_400", func(t *testing.T) {
		svc := &mockCategoryGoalService{}
		svc.On("Upsert", mock.Anything, id, mock.Anything).Return(nil, model.ErrInvalidGoal)
		assert.Equal(t, http.StatusBadRequest, run(svc, goal))
	})
	t.Run("missing_category_returns_404", func(t *testing.T) {
		svc := &mockCategoryGoalService{}
		svc.On("Upsert", mock.Anything, id, mock.Anything).Return(nil, service.ErrCategoryNotFound)
		assert.Equal(t, http.StatusNotFound, run(svc, goal))
	})
}

func TestCategoryGoalHandler_Delete(t *testing.T) {
	id := uuid.New()
	run := func(svc *mockCategoryGoalService) int {
		w, c := makeReq("DELETE", "/categories/"+id.String()+"/goal", nil)
		c.Params = gin.Params{{Key: "id", Value: id.String()}}
		NewCategoryGoalHandler(svc).Delete(c)
		return w.Code
	}
	t.Run("deletes_goal", func(t *testing.T) {
		svc := &mockCategoryGoalService{}
		svc.On("Delete", mock.Anything, id).Return(nil)
		assert.Equal(t, http.StatusOK, run(svc))
	})
	t.Run("no_goal_returns_404", func(t *testing.T) {
		svc := &mockCategoryGoalService{}
		svc.On("Delete", mock.Anything, id).Return(service.ErrCategoryGoalNotFound)
		assert.Equal(t, http.StatusNotFound, run(svc))
	})
}
