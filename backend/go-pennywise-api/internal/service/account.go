package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type AccountService interface {
	GetAll(ctx context.Context) ([]model.Account, error)
	Create(ctx context.Context, account model.Account) error
}

type accountService struct {
	repo repository.AccountRepository
}

func NewAccountService(r repository.AccountRepository) AccountService {
	return &accountService{repo: r}
}

func (s *accountService) GetAll(ctx context.Context) ([]model.Account, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *accountService) Create(ctx context.Context, account model.Account) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	account.BudgetID = budgetId
	return s.repo.Create(ctx, account)
}
