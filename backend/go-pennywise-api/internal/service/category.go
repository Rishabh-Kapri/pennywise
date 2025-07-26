package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"
)

type CategoryService interface {
	GetAll(ctx context.Context) ([]model.Category, error)
	Create(ctx context.Context, category model.Category) (error)
}

type categoryService struct {
	repo repository.CategoryRepository
}

func NewCategoryService(r repository.CategoryRepository) CategoryService {
	return &categoryService{repo: r}
}

func (s *categoryService) GetAll(ctx context.Context) ([]model.Category, error) {
	budgetId, _ := ctx.Value("budgetId").(string)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *categoryService) Create(ctx context.Context, category model.Category) (error) {
	budgetId, _ := ctx.Value("budgetId").(string)
	category.BudgetID = budgetId
	return s.repo.Create(ctx, category)
}
