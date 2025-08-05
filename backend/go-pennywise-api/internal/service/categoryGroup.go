package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type CategoryGroupService interface {
	GetAll(ctx context.Context) ([]model.CategoryGroup, error)
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
