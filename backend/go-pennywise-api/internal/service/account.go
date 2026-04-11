package service

import (
	"context"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/pkg"

	"github.com/jackc/pgx/v5"
)

type AccountService interface {
	GetAll(ctx context.Context) ([]model.Account, error)
	Search(ctx context.Context, query string) ([]model.Account, error)
	Create(ctx context.Context, account model.Account) (*model.Account, error)
}

type accountService struct {
	repo      repository.AccountRepository
	payeeRepo repository.PayeesRepository
}

func NewAccountService(r repository.AccountRepository, payeeRepo repository.PayeesRepository) AccountService {
	return &accountService{repo: r, payeeRepo: payeeRepo}
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

	var createdAcc *model.Account
	err := utils.WithTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		// 1. create account
		acc, err := s.repo.Create(ctx, tx, account)
		if err != nil {
			return fmt.Errorf("accountService.Create; error creating account: %v", err)
		}
		createdAcc = acc

		// 2. create transfer payee for the account
		transferPayee := model.Payee{
			Name:              "Transfer : " + account.Name,
			BudgetID:          budgetId,
			TransferAccountID: &createdAcc.ID,
		}
		createdPayee, err := s.payeeRepo.Create(ctx, tx, transferPayee)
		if err != nil {
			return fmt.Errorf("accountService.Create; error creating transfer payee: %v", err)
		}

		// 3. update account with transfer payee id
		err = s.repo.UpdateTransferPayee(ctx, tx, createdAcc.ID, createdPayee.ID)
		if err != nil {
			return fmt.Errorf("accountService.Create; error updating transfer payee: %v", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdAcc, nil
}
