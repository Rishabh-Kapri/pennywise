package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"
)

type BudgetService interface {
	GetAll(ctx context.Context) ([]model.Budget, error)
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
