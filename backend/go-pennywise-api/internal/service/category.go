package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type CategoryService interface {
	GetAll(ctx context.Context) ([]model.Category, error)
	GetById(ctx context.Context, id uuid.UUID) (*model.Category, error)
	Create(ctx context.Context, category model.Category) error
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type categoryService struct {
	repo repository.CategoryRepository
}

func NewCategoryService(r repository.CategoryRepository) CategoryService {
	return &categoryService{repo: r}
}

func (s *categoryService) GetAll(ctx context.Context) ([]model.Category, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *categoryService) GetById(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetById(ctx, budgetId, id)
}

func (s *categoryService) Create(ctx context.Context, category model.Category) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	category.BudgetID = budgetId
	return s.repo.Create(ctx, category)
}

func (s *categoryService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.DeleteById(ctx, budgetId, id)
}
