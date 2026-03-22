package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"
	utils "pennywise-api/pkg"
)

type AccountService interface {
	GetAll(ctx context.Context) ([]model.Account, error)
	Search(ctx context.Context, query string) ([]model.Account, error)
	Create(ctx context.Context, account model.Account) (*model.Account, error)
}

type accountService struct {
	repo repository.AccountRepository
}

func NewAccountService(r repository.AccountRepository) AccountService {
	return &accountService{repo: r}
}

func (s *accountService) GetAll(ctx context.Context) ([]model.Account, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *accountService) Search(ctx context.Context, query string) ([]model.Account, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Search(ctx, budgetId, query)
}

func (s *accountService) Create(ctx context.Context, account model.Account) (*model.Account, error) {
	budgetId := utils.MustBudgetID(ctx)
	account.BudgetID = budgetId
	return s.repo.Create(ctx, account)
}
