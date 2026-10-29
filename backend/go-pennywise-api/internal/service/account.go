package service

import (
	"context"
	"errors"
	"strings"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AccountService interface {
	GetAll(ctx context.Context) ([]model.Account, error)
	Search(ctx context.Context, query string) ([]model.Account, error)
	Create(ctx context.Context, account model.Account) (*model.Account, error)
	Update(ctx context.Context, id uuid.UUID, account model.Account) (*model.Account, error)
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

// validAccountTypes mirrors the account types the clients offer.
var validAccountTypes = map[string]bool{
	"checking": true, "savings": true, "creditCard": true,
	"asset": true, "liability": true,
	"loan": true, "mortgage": true, "autoLoan": true,
	"studentLoan": true, "personalLoan": true, "medicalDebt": true,
}

func (s *accountService) Create(ctx context.Context, account model.Account) (*model.Account, error) {
	account.Name = strings.TrimSpace(account.Name)
	if account.Name == "" {
		return nil, errors.New("account name is required")
	}
	if !validAccountTypes[account.Type] {
		return nil, errors.New("invalid account type: " + account.Type)
	}
	account.Bank = strings.ToLower(strings.TrimSpace(account.Bank))
	account.Suffix = strings.TrimSpace(account.Suffix)

	budgetId := utils.MustBudgetID(ctx)
	account.BudgetID = budgetId
	if err := s.checkSuffix(ctx, budgetId, uuid.Nil, account.Suffix); err != nil {
		return nil, err
	}

	var createdAcc *model.Account
	err := utils.WithTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		// 1. create account
		acc, err := s.repo.Create(ctx, tx, account)
		if err != nil {
			return errs.Wrap(errs.CodeAccountCreateFailed, "error creating account", err)
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
			return errs.Wrap(errs.CodeAccountCreateFailed, "error creating transfer payee", err)
		}

		// 3. update account with transfer payee id
		err = s.repo.UpdateTransferPayee(ctx, tx, createdAcc.ID, createdPayee.ID)
		if err != nil {
			return errs.Wrap(errs.CodeAccountCreateFailed, "error updating transfer payee", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdAcc, nil
}

// checkSuffix validates an account's last-digits suffix: 2-6 digits, and not
// already used by another account in the budget (statement emails are matched
// to accounts by it). An empty suffix is fine.
func (s *accountService) checkSuffix(ctx context.Context, budgetId, selfId uuid.UUID, suffix string) error {
	if suffix == "" {
		return nil
	}
	if len(suffix) < 2 || len(suffix) > 6 || strings.Trim(suffix, "0123456789") != "" {
		return errors.New("account suffix must be 2-6 digits")
	}
	existing, err := s.repo.GetBySuffix(ctx, budgetId, suffix)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != selfId {
		return errors.New("another account already uses suffix " + suffix)
	}
	return nil
}

// Update edits an account's name, bank, suffix and closed flag. The type is
// fixed once created. Renaming also renames its transfer payee.
func (s *accountService) Update(ctx context.Context, id uuid.UUID, account model.Account) (*model.Account, error) {
	account.Name = strings.TrimSpace(account.Name)
	if account.Name == "" {
		return nil, errors.New("account name is required")
	}
	account.Bank = strings.ToLower(strings.TrimSpace(account.Bank))
	account.Suffix = strings.TrimSpace(account.Suffix)

	budgetId := utils.MustBudgetID(ctx)
	if err := s.checkSuffix(ctx, budgetId, id, account.Suffix); err != nil {
		return nil, err
	}

	var updated *model.Account
	var renamedFrom string
	err := utils.WithTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		current, err := s.repo.GetById(ctx, tx, budgetId, id)
		if err != nil {
			return errors.New("account not found")
		}
		renamedFrom = current.Name
		updated, err = s.repo.Update(ctx, tx, budgetId, id, account)
		return err
	})
	if err != nil {
		return nil, err
	}

	if updated.TransferPayeeID != nil && renamedFrom != updated.Name {
		err := s.payeeRepo.Update(ctx, budgetId, *updated.TransferPayeeID, model.Payee{Name: "Transfer : " + updated.Name})
		if err != nil {
			return nil, errs.Wrap(errs.CodeAccountCreateFailed, "account updated but its transfer payee could not be renamed", err)
		}
	}
	return updated, nil
}
