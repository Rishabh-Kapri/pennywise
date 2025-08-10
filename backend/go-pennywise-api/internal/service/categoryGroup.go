package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type CategoryGroupService interface {
	GetAll(ctx context.Context) ([]model.CategoryGroup, error)
	Create(ctx context.Context, categoryGroup model.CategoryGroup) error
	Update(ctx context.Context, id uuid.UUID, categoryGroup model.CategoryGroup) error
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type categoryGroupService struct {
	repo repository.CategoryGroupRepository
}

func NewCategoryGroupService(r repository.CategoryGroupRepository) CategoryGroupService {
	return &categoryGroupService{repo: r}
}

func (s *categoryGroupService) GetAll(ctx context.Context) ([]model.CategoryGroup, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *categoryGroupService) Create(ctx context.Context, categoryGroup model.CategoryGroup) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	categoryGroup.BudgetID = budgetId
	return s.repo.Create(ctx, categoryGroup)
}

func (s *categoryGroupService) Update(ctx context.Context, id uuid.UUID, categoryGroup model.CategoryGroup) error  {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Update(ctx, budgetId, id, categoryGroup)
}

func (s *categoryGroupService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.DeleteById(ctx, budgetId, id)
}
