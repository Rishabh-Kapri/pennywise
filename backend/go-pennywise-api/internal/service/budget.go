package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type BudgetService interface {
	GetAll(ctx context.Context) ([]model.Budget, error)
	Create(ctx context.Context, name string) error
	UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error
}

type budgetService struct {
	repo repository.BudgetRepository
}

func NewBudgetService(repo repository.BudgetRepository) BudgetService {
	return &budgetService{repo}
}

func (s *budgetService) GetAll(ctx context.Context) ([]model.Budget, error) {
	return s.repo.GetAll(ctx)
}
func (s *budgetService) Create(ctx context.Context, name string) error {
	return s.repo.Create(ctx, name)
}

func (s *budgetService) UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error {
	return s.repo.UpdateById(ctx, id, budget)
}
