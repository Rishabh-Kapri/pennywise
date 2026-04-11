package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"
	utils "pennywise-api/pkg"
)

type UserService interface {
	Search(ctx context.Context, query string) ([]model.User, error)
	Update(ctx context.Context, user model.User) (*model.User, error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(r repository.UserRepository) UserService {
	return &userService{repo: r}
}

func (s *userService) Search(ctx context.Context, query string) ([]model.User, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Search(ctx, budgetId, query)
}

func (s *userService) Update(ctx context.Context, user model.User) (*model.User, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Update(ctx, budgetId, user)
}
