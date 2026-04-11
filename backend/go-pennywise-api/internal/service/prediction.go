package service

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/pkg"

	"github.com/google/uuid"
)

type PredictionService interface {
	GetAll(ctx context.Context) ([]model.Prediction, error)
	Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error)
	Update(ctx context.Context, id uuid.UUID, prediction model.Prediction) error
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type predictionService struct {
	repo repository.PredictionRepository
}

func NewPredictionService(r repository.PredictionRepository) PredictionService {
	return &predictionService{repo: r}
}

func (s *predictionService) GetAll(ctx context.Context) ([]model.Prediction, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *predictionService) Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error) {
	budgetId := utils.MustBudgetID(ctx)
	prediction.BudgetID = budgetId
	return s.repo.Create(ctx, prediction)
}

func (s *predictionService) Update(ctx context.Context, id uuid.UUID, prediction model.Prediction) error {
	// budgetId := utils.MustBudgetID(ctx)
	// INFO: prediction update for now will only be done through transactions update
	// return s.repo.Update(ctx, budgetId, id, prediction)
	return nil
}

func (s *predictionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	// budgetId := utils.MustBudgetID(ctx)
	// return s.repo.Delete(ctx, budgetId, id)
	return nil
}
