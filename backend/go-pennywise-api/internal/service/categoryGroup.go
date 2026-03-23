package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	utils "pennywise-api/pkg"

	"github.com/google/uuid"
)

type CategoryGroupService interface {
	GetAll(ctx context.Context, month string) ([]model.CategoryGroup, error)
	Create(ctx context.Context, categoryGroup model.CategoryGroup) (*model.CategoryGroup, error)
	Update(ctx context.Context, id uuid.UUID, categoryGroup model.CategoryGroup) error
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type categoryGroupService struct {
	repo repository.CategoryGroupRepository
}

func NewCategoryGroupService(r repository.CategoryGroupRepository) CategoryGroupService {
	return &categoryGroupService{repo: r}
}

func (s *categoryGroupService) GetAll(ctx context.Context, month string) ([]model.CategoryGroup, error) {
	budgetId := utils.MustBudgetID(ctx)
	groups, err := s.repo.GetAll(ctx, budgetId)
	if err != nil {
		return nil, err
	}
	utils.Logger(ctx).Debug("listing category groups", "month", month)
	if month != "" {
		for _, group := range groups {
			group.Balance = utils.FillCarryForward(group.Balance, month)
			for _, category := range group.Categories {
				category.Balance = utils.FillCarryForward(category.Balance, month)
			}
		}
	}
	return groups, nil
}

func (s *categoryGroupService) Create(ctx context.Context, categoryGroup model.CategoryGroup) (*model.CategoryGroup, error) {
	budgetId := utils.MustBudgetID(ctx)
	categoryGroup.BudgetID = budgetId
	return s.repo.Create(ctx, nil, categoryGroup)
}

func (s *categoryGroupService) Update(ctx context.Context, id uuid.UUID, categoryGroup model.CategoryGroup) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Update(ctx, budgetId, id, categoryGroup)
}

func (s *categoryGroupService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.DeleteById(ctx, budgetId, id)
}
