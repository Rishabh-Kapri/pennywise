package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type PredictionService interface {
	GetAll(ctx context.Context) ([]model.Prediction, error)
	Create(ctx context.Context, prediction model.Prediction) error
}

type predictionService struct {
	repo repository.PredictionRepository
}

func NewPredictionService(r repository.PredictionRepository) PredictionService {
	return &predictionService{repo: r}
}

func (s *predictionService) GetAll(ctx context.Context) ([]model.Prediction, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *predictionService) Create(ctx context.Context, prediction model.Prediction) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	prediction.BudgetID = budgetId
	return s.repo.Create(ctx, prediction)
}
