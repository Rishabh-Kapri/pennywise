package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
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
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Search(ctx, budgetId, query)
}

func (s *userService) Update(ctx context.Context, user model.User) (*model.User, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Update(ctx, budgetId, user)
}
