package service

import (
	"context"
	"log"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	utils "pennywise-api/pkg"

	"github.com/google/uuid"
)

type CategoryGroupService interface {
	GetAll(ctx context.Context, month string) ([]model.CategoryGroup, error)
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

func (s *categoryGroupService) GetAll(ctx context.Context, month string) ([]model.CategoryGroup, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	groups, err := s.repo.GetAll(ctx, budgetId)
	if err != nil {
		return nil, err
	}
	log.Printf("%v", month)
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

func (s *categoryGroupService) Create(ctx context.Context, categoryGroup model.CategoryGroup) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	categoryGroup.BudgetID = budgetId
	return s.repo.Create(ctx, categoryGroup)
}

func (s *categoryGroupService) Update(ctx context.Context, id uuid.UUID, categoryGroup model.CategoryGroup) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Update(ctx, budgetId, id, categoryGroup)
}

func (s *categoryGroupService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.DeleteById(ctx, budgetId, id)
}
