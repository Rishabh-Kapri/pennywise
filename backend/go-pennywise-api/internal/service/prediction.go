package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

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
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *predictionService) Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	prediction.BudgetID = budgetId
	return s.repo.Create(ctx, prediction)
}

func (s *predictionService) Update(ctx context.Context, id uuid.UUID, prediction model.Prediction) error {
   budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
   return s.repo.Update(ctx, budgetId, id, prediction)
}

func (s *predictionService) DeleteById(ctx context.Context, id uuid.UUID) error {
   budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
   return s.repo.DeleteById(ctx, budgetId, id)
}
