package service

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

type TagService interface {
	GetAll(ctx context.Context) ([]model.Tag, error)
	Search(ctx context.Context, query string) ([]model.Tag, error)
	GetById(ctx context.Context, id uuid.UUID) (*model.Tag, error)
	Create(ctx context.Context, tag model.Tag) (*model.Tag, error)
	Update(ctx context.Context, id uuid.UUID, tag model.Tag) error
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type tagService struct {
	repo repository.TagRepository
}

func NewTagService(repo repository.TagRepository) TagService {
	return &tagService{repo}
}

func (s *tagService) GetAll(ctx context.Context) ([]model.Tag, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *tagService) Search(ctx context.Context, query string) ([]model.Tag, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Search(ctx, budgetId, query)
}

func (s *tagService) GetById(ctx context.Context, id uuid.UUID) (*model.Tag, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetById(ctx, budgetId, id)
}

func (s *tagService) Create(ctx context.Context, tag model.Tag) (*model.Tag, error) {
	budgetId := utils.MustBudgetID(ctx)
	tag.BudgetID = budgetId
	return s.repo.Create(ctx, nil, tag)
}

func (s *tagService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.DeleteById(ctx, budgetId, id)
}

func (s *tagService) Update(ctx context.Context, id uuid.UUID, tag model.Tag) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Update(ctx, budgetId, id, tag)
}
