package service

import (
	"context"
	"errors"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
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
	dbTxn     repository.DBTxn
}

func NewAccountService(r repository.AccountRepository, payeeRepo repository.PayeesRepository, dbTxn repository.DBTxn) AccountService {
	return &accountService{dbTxn: dbTxn, repo: r, payeeRepo: payeeRepo}
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
	account.Name = strings.TrimSpace(account.Name)
	if account.Name == "" {
		return nil, errors.New("account name is required")
	}
	if !model.ValidAccountType(account.Type) {
		return nil, errors.New("invalid account type: " + account.Type)
	}
	account.Bank = strings.ToLower(strings.TrimSpace(account.Bank))
	account.Suffix = strings.TrimSpace(account.Suffix)

	budgetId := utils.MustBudgetID(ctx)
	account.BudgetID = budgetId
	if err := s.checkSuffix(ctx, budgetId, uuid.Nil, account.Suffix); err != nil {
		return nil, err
	}

	var created *model.Account
	err := s.dbTxn.Do(ctx, func(ctx context.Context) error {
		var err error
		created, _, err = createAccount(ctx, s.repo, s.payeeRepo, account)
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// createAccount creates an account and the transfer payee that moves money
// into it. Run it inside a database transaction.
func createAccount(
	ctx context.Context,
	accounts repository.AccountRepository,
	payees repository.PayeesRepository,
	account model.Account,
) (*model.Account, *model.Payee, error) {
	created, err := accounts.Create(ctx, account)
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeAccountCreateFailed, "error creating account", err)
	}
	transferPayee, err := payees.Create(ctx, model.Payee{
		Name:              model.TransferPayeeName(account.Name),
		BudgetID:          account.BudgetID,
		TransferAccountID: &created.ID,
	})
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeAccountCreateFailed, "error creating transfer payee", err)
	}
	if err := accounts.UpdateTransferPayee(ctx, created.ID, transferPayee.ID); err != nil {
		return nil, nil, errs.Wrap(errs.CodeAccountCreateFailed, "error updating transfer payee", err)
	}
	return created, transferPayee, nil
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
	if errors.Is(err, repository.ErrNotFound) {
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
	err := s.dbTxn.Do(ctx, func(ctx context.Context) error {
		current, err := s.repo.GetById(ctx, budgetId, id)
		if err != nil {
			return errors.New("account not found")
		}
		renamedFrom = current.Name
		updated, err = s.repo.Update(ctx, budgetId, id, account)
		return err
	})
	if err != nil {
		return nil, err
	}

	if updated.TransferPayeeID != nil && renamedFrom != updated.Name {
		err := s.payeeRepo.Update(ctx, budgetId, *updated.TransferPayeeID, model.Payee{Name: model.TransferPayeeName(updated.Name)})
		if err != nil {
			return nil, errs.Wrap(errs.CodeAccountCreateFailed, "account updated but its transfer payee could not be renamed", err)
		}
	}
	return updated, nil
}
