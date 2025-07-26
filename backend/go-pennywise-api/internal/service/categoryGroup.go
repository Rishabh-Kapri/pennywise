package service

import (
	"context"
	"errors"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"
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
	budgetId, ok := ctx.Value("budgetId").(string)
	if !ok || budgetId == "" {
		return nil, errors.New("Missing budgetId in context")
	}
	return s.repo.GetAll(ctx, budgetId)
}
