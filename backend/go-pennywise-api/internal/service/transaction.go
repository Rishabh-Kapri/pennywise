package service

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/repository"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var withTx = utils.WithTx

type TransactionService interface {
	GetAll(ctx context.Context) ([]model.Transaction, error)
	GetAllNormalized(ctx context.Context, accountId *uuid.UUID) ([]model.Transaction, error)
	// GetById(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	Update(ctx context.Context, id uuid.UUID, txn model.Transaction) error
	Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error)
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type transactionService struct {
	repo           repository.TransactionRepository
	budgetRepo     repository.BudgetRepository
	predictionRepo repository.PredictionRepository
	accountRepo    repository.AccountRepository
	payeeRepo      repository.PayeesRepository
	categoryRepo   repository.CategoryRepository
	mbService      MonthlyBudgetService
}

func NewTransactionService(
	r repository.TransactionRepository,
	budgetRepo repository.BudgetRepository,
	predictionRepo repository.PredictionRepository,
	accountRepo repository.AccountRepository,
	payeeRepo repository.PayeesRepository,
	catRepo repository.CategoryRepository,
	mbService MonthlyBudgetService,
) TransactionService {
	return &transactionService{
		repo:           r,
		budgetRepo:     budgetRepo,
		predictionRepo: predictionRepo,
		accountRepo:    accountRepo,
		payeeRepo:      payeeRepo,
		categoryRepo:   catRepo,
		mbService:      mbService,
	}
}

// internal method
func (s *transactionService) updatePrediction(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnId uuid.UUID, txn model.Transaction, account model.Account, payee model.Payee) error {
	prediction, err := s.predictionRepo.GetByTxnIdTx(ctx, tx, budgetId, txnId)
	if err != nil {
		return errs.Wrap(errs.CodePredictionLookupFailed, "error getting prediction", err)
	}
	if prediction == nil {
		logger.Logger(ctx).Info("prediction not found for txn", "txnId", txnId)
		return nil
	}
	logger.Logger(ctx).Debug("prediction", "prediction", prediction.String())

	logger.Logger(ctx).Debug("transaction", "txn", txn.String())

	var category *model.Category
	if txn.CategoryID != nil {
		category, err = s.categoryRepo.GetByIdSimplifiedTx(ctx, tx, budgetId, *txn.CategoryID)
		if err != nil {
			return errs.Wrap(errs.CodeCategoryLookupFailed, "error getting category", err)
		}
	}

	// if the prediction is not the same as the existing one, update it
	trueVal := true
	falseVal := false
	needsUpdate := false

	accountName := account.Name
	payeeName := payee.Name

	if prediction.Account != nil && *prediction.Account != accountName {
		prediction.HasUserCorrected = &trueVal
		corrected := account.Name
		prediction.UserCorrectedAccount = &corrected
		needsUpdate = true
	}
	if prediction.Payee != nil && *prediction.Payee != payeeName {
		prediction.HasUserCorrected = &trueVal
		corrected := payee.Name
		prediction.UserCorrectedPayee = &corrected
		needsUpdate = true
	}
	if prediction.Category != nil && category != nil && *prediction.Category != category.Name {
		prediction.HasUserCorrected = &trueVal
		corrected := category.Name
		prediction.UserCorrectedCategory = &corrected
		needsUpdate = true
	}

	// If no mismatch found but prediction was previously marked as corrected, clear it
	if !needsUpdate && prediction.HasUserCorrected != nil && *prediction.HasUserCorrected {
		prediction.HasUserCorrected = &falseVal
		needsUpdate = true
	}

	if needsUpdate {
		logger.Logger(ctx).Info("updating prediction", "txnId", txnId)
		err = s.predictionRepo.Update(ctx, tx, budgetId, prediction.ID, *prediction)
		if err != nil {
			return errs.Wrap(errs.CodePredictionUpdateFailed, "failed to update prediction", err)
		}
	}
	return nil
}

// validateTransactionPayload validates the payload of a transaction
func (s *transactionService) validateTransactionPayload(txn model.Transaction, budgetID uuid.UUID) error {
	if txn.BudgetID != budgetID {
		return errs.New(errs.CodeInvalidArgument, "transaction is not for this budget")
	}
	if txn.AccountID == nil {
		return errs.New(errs.CodeInvalidArgument, "account_id is required")
	}
	if txn.PayeeID == nil {
		return errs.New(errs.CodeInvalidArgument, "payee_id is required")
	}
	if err := txn.Date.Valid(); err != nil {
		return err
	}

	return nil
}

// loadDependencies loads the budget, account, and payee for the transaction
func (s *transactionService) loadDependencies(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txn model.Transaction) (budget *model.Budget, account *model.Account, payee *model.Payee, err error) {
	budget, err = s.budgetRepo.GetById(ctx, tx, budgetId)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodeBudgetLookupFailed, "error fetching budget", err)
	}

	account, err = s.accountRepo.GetById(ctx, tx, budgetId, *txn.AccountID)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodeAccountLookupFailed, "error getting account", err)
	}

	payee, err = s.payeeRepo.GetByIdTx(ctx, tx, budgetId, *txn.PayeeID)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodePayeeLookupFailed, "error getting payee", err)
	}

	return budget, account, payee, nil
}

// validate the category of the transaction
// for budget transfers, the category should be nil
func (s *transactionService) validateCategory(categoryID *uuid.UUID, inflowCategoryID uuid.UUID, account model.Account, payee model.Payee, amount float64) error {
	// budget -> budget transfers don't have a category
	if payee.TransferAccountID != nil {
		if account.Type == "savings" || account.Type == "checking" || account.Type == "creditCard" {
			if categoryID != nil {
				return errs.New(errs.CodeInvalidArgument, "category is not allowed for budget transfers")
			}
		}
	}
	if categoryID != nil && *categoryID == inflowCategoryID && amount < 0 {
		return errs.New(errs.CodeInvalidArgument, "negative inflow category amounts are not allowed")
	}
	return nil
}

// createCounterpartTxn creates the counterpart transaction for a transfer
func (s *transactionService) createCounterpartTxn(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, parentId uuid.UUID, txn model.Transaction, account model.Account, payee model.Payee) (uuid.UUID, error) {
	counterpart := model.Transaction{
		BudgetID:              budgetId,
		AccountID:             payee.TransferAccountID,
		PayeeID:               account.TransferPayeeID,
		CategoryID:            nil,
		Amount:                -txn.Amount,
		Date:                  txn.Date,
		Note:                  txn.Note,
		Source:                txn.Source,
		TransferAccountID:     txn.AccountID,
		TransferTransactionID: &parentId,
	}
	created, err := s.repo.Create(ctx, tx, counterpart)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.CodeTransferCreateFailed, "error creating transfer transaction", err)
	}
	if len(created) == 0 {
		return uuid.Nil, errs.New(errs.CodeTransferNotCreated, "no transfer transaction was created")
	}
	return created[0].ID, nil
}

// sideEffectInput holds the context for applying side effects to a transaction.
// oldTxn == nil means create, newTxn == nil means delete, both non-nil means update.
type sideEffectInput struct {
	budgetId uuid.UUID
	oldTxn   *model.Transaction // nil for create
	newTxn   *model.Transaction // nil for delete
	budget   *model.Budget      // required for create (inflow category check)
	account  *model.Account     // nil for delete
	payee    *model.Payee       // nil for delete
}

// applySideEffects applies all side effects (carryovers, transfers, predictions)
// for a transaction create, update, or delete in a single unified method.
func (s *transactionService) applySideEffects(ctx context.Context, tx pgx.Tx, input sideEffectInput) error {
	isCreate := input.oldTxn == nil && input.newTxn != nil
	isUpdate := input.oldTxn != nil && input.newTxn != nil
	isDelete := input.oldTxn != nil && input.newTxn == nil

	// --- Carryovers ---
	switch {
	case isCreate:
		if input.newTxn.CategoryID != nil && *input.newTxn.CategoryID != input.budget.Metadata.InflowCategoryID {
			monthKey := utils.GetMonthKey(input.newTxn.Date.String())
			if err := s.mbService.UpsertCarryover(ctx, tx, input.budgetId, *input.newTxn.CategoryID, monthKey, input.newTxn.Amount); err != nil {
				return err
			}
		}
	case isUpdate:
		if err := s.mbService.UpdateCarryovers(ctx, tx, input.budgetId, input.oldTxn, input.newTxn, input.budget.Metadata.InflowCategoryID); err != nil {
			return err
		}
	case isDelete:
		if input.oldTxn.CategoryID != nil && (input.budget == nil || *input.oldTxn.CategoryID != input.budget.Metadata.InflowCategoryID) {
			monthKey := utils.GetMonthKey(input.oldTxn.Date.String())
			if err := s.mbService.UpsertCarryover(ctx, tx, input.budgetId, *input.oldTxn.CategoryID, monthKey, -input.oldTxn.Amount); err != nil {
				return err
			}
		}
	}

	// --- Transfers ---
	switch {
	case isCreate:
		if input.payee != nil && input.payee.TransferAccountID != nil {
			createdId, err := s.createCounterpartTxn(ctx, tx, input.budgetId, input.newTxn.ID, *input.newTxn, *input.account, *input.payee)
			if err != nil {
				return err
			}
			input.newTxn.TransferAccountID = input.payee.TransferAccountID
			input.newTxn.TransferTransactionID = &createdId
			if err = s.repo.Update(ctx, tx, input.budgetId, input.newTxn.ID, *input.newTxn); err != nil {
				return errs.Wrap(errs.CodeTransactionUpdateFailed, "error linking transfer transaction", err)
			}
		}
	case isUpdate:
		if err := s.reconcileTransfer(ctx, tx, input.budgetId, *input.oldTxn, input.newTxn, *input.account, *input.payee); err != nil {
			return err
		}
	case isDelete:
		if input.oldTxn.TransferTransactionID != nil {
			if err := s.repo.DeleteById(ctx, tx, input.budgetId, *input.oldTxn.TransferTransactionID); err != nil {
				return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting transfer transaction", err)
			}
		}
	}

	// --- Predictions ---
	switch {
	case isUpdate && input.oldTxn.Source == "MLP":
		if err := s.updatePrediction(ctx, tx, input.budgetId, input.oldTxn.ID, *input.newTxn, *input.account, *input.payee); err != nil {
			return errs.Wrap(errs.CodeTransactionUpdateFailed, "error updating prediction", err)
		}
	case isDelete && input.oldTxn.Source == "MLP":
		prediction, err := s.predictionRepo.GetByTxnIdTx(ctx, tx, input.budgetId, input.oldTxn.ID)
		if err != nil {
			return errs.Wrap(errs.CodePredictionLookupFailed, "error getting prediction", err)
		}
		if prediction != nil {
			if err = s.predictionRepo.DeleteByTxnId(ctx, tx, input.budgetId, input.oldTxn.ID); err != nil {
				return errs.Wrap(errs.CodePredictionDeleteFailed, "error deleting prediction", err)
			}
		}
	}

	return nil
}

func (s *transactionService) reconcileTransfer(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, foundTxn model.Transaction, newTxn *model.Transaction, account model.Account, payee model.Payee) error {
	// reconcile transfer transactions
	wasTransfer := foundTxn.TransferTransactionID != nil
	isTransfer := payee.TransferAccountID != nil
	samePayee := foundTxn.PayeeID != nil && newTxn.PayeeID != nil && *foundTxn.PayeeID == *newTxn.PayeeID

	switch {
	case wasTransfer && !isTransfer:
		// transfer → regular: delete counterpart, clear fields
		logger.Logger(ctx).Info("converting transfer to regular, deleting counterpart", "transferTxnId", *foundTxn.TransferTransactionID)
		if err := s.repo.DeleteById(ctx, tx, budgetId, *foundTxn.TransferTransactionID); err != nil {
			return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting transfer transaction", err)
		}
		newTxn.TransferAccountID = nil
		newTxn.TransferTransactionID = nil

	case !wasTransfer && isTransfer:
		// regular → transfer: create counterpart
		logger.Logger(ctx).Info("converting regular to transfer, creating counterpart")
		createdId, err := s.createCounterpartTxn(ctx, tx, budgetId, foundTxn.ID, *newTxn, account, payee)
		if err != nil {
			return err
		}
		newTxn.TransferAccountID = payee.TransferAccountID
		newTxn.TransferTransactionID = &createdId

	case wasTransfer && isTransfer && !samePayee:
		// transfer → different transfer: delete old counterpart, create new
		logger.Logger(ctx).Info("changing transfer destination, recreating counterpart")
		if err := s.repo.DeleteById(ctx, tx, budgetId, *foundTxn.TransferTransactionID); err != nil {
			return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting old transfer transaction", err)
		}
		createdId, err := s.createCounterpartTxn(ctx, tx, budgetId, foundTxn.ID, *newTxn, account, payee)
		if err != nil {
			return err
		}
		newTxn.TransferAccountID = payee.TransferAccountID
		newTxn.TransferTransactionID = &createdId

	case wasTransfer && isTransfer && samePayee:
		// same transfer: update counterpart with new amount/date/note
		logger.Logger(ctx).Info("updating existing transfer counterpart", "transferTxnId", *foundTxn.TransferTransactionID)
		counterpart := model.Transaction{
			BudgetID:              budgetId,
			AccountID:             payee.TransferAccountID,
			PayeeID:               account.TransferPayeeID,
			CategoryID:            nil,
			Amount:                -newTxn.Amount,
			Date:                  newTxn.Date,
			Note:                  newTxn.Note,
			Source:                newTxn.Source,
			TransferAccountID:     newTxn.AccountID,
			TransferTransactionID: &foundTxn.ID,
		}
		if err := s.repo.Update(ctx, tx, budgetId, *foundTxn.TransferTransactionID, counterpart); err != nil {
			return errs.Wrap(errs.CodeTransactionUpdateFailed, "error updating transfer counterpart", err)
		}
		newTxn.TransferAccountID = foundTxn.TransferAccountID
		newTxn.TransferTransactionID = foundTxn.TransferTransactionID
	}
	return nil
}

func (s *transactionService) GetAll(ctx context.Context) ([]model.Transaction, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId, nil)
}

func (s *transactionService) GetAllNormalized(ctx context.Context, accountId *uuid.UUID) ([]model.Transaction, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAllNormalized(ctx, budgetId, accountId)
}

func (s *transactionService) Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error) {
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetID := utils.MustBudgetID(ctx)
	txn.BudgetID = budgetID

	if err := s.validateTransactionPayload(txn, budgetID); err != nil {
		return nil, err
	}

	var createdTxn []model.Transaction
	err := withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		var err error

		budget, account, payee, err := s.loadDependencies(txCtx, tx, budgetID, txn)
		if err != nil {
			return err
		}

		if err = s.validateCategory(txn.CategoryID, budget.Metadata.InflowCategoryID, *account, *payee, txn.Amount); err != nil {
			return err
		}

		// clear transfer fields in case they are set
		txn.TransferAccountID = nil
		txn.TransferTransactionID = nil

		createdTxn, err = s.repo.Create(txCtx, tx, txn)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionCreateFailed, "failed to create transaction", err)
		}
		if len(createdTxn) == 0 {
			return errs.New(errs.CodeTransactionNotCreated, "no transaction was created")
		}

		txn.ID = createdTxn[0].ID

		if err = s.applySideEffects(txCtx, tx, sideEffectInput{
			budgetId: budgetID,
			oldTxn:   nil,
			newTxn:   &txn,
			budget:   budget,
			account:  account,
			payee:    payee,
		}); err != nil {
			return err
		}

		// Reload to pick up any mutations from side effects (e.g., transfer linking)
		final, err := s.repo.GetByIdTx(txCtx, tx, budgetID, txn.ID)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionLookupFailed, "error reloading created transaction", err)
		}
		createdTxn[0] = *final

		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdTxn, nil
}

func (s *transactionService) Update(ctx context.Context, id uuid.UUID, txn model.Transaction) error {
	// Create a shorter context for each individual transaction attempt
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetId := utils.MustBudgetID(ctx)
	logger.Logger(ctx).Info("updating transaction", "id", id)

	// Create a copy of the transaction to update
	toUpdate := txn

	if err := s.validateTransactionPayload(toUpdate, budgetId); err != nil {
		return err
	}

	return withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		foundTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, id)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting transaction", err)
		}
		if foundTxn == nil {
			return errs.New(errs.CodeTransactionLookupFailed, "transaction not found for id %v", id)
		}
		if foundTxn.ID != id {
			return errs.New(errs.CodeTransactionLookupFailed, "transaction id mismatch")
		}

		toUpdate.ID = id
		same := foundTxn.Compare(&toUpdate)
		if same {
			logger.Logger(txCtx).Info("transaction is the same as the existing transaction, skipping update")
			return nil
		}

		// fetch updated txn account and payee
		budget, account, payee, err := s.loadDependencies(txCtx, tx, budgetId, toUpdate)

		err = s.validateCategory(toUpdate.CategoryID, budget.Metadata.InflowCategoryID, *account, *payee, toUpdate.Amount)
		if err != nil {
			return err
		}

		if err = s.applySideEffects(txCtx, tx, sideEffectInput{
			budgetId: budgetId,
			oldTxn:   foundTxn,
			newTxn:   &toUpdate,
			budget:   budget,
			account:  account,
			payee:    payee,
		}); err != nil {
			return err
		}

		if err = s.repo.Update(txCtx, tx, budgetId, id, toUpdate); err != nil {
			return errs.Wrap(errs.CodeTransactionUpdateFailed, "error updating transaction", err)
		}

		return nil
	})
}

func (s *transactionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetId := utils.MustBudgetID(ctx)
	logger.Logger(ctx).Info("deleting transaction", "id", id)

	return withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		foundTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, id)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting transaction", err)
		}
		if foundTxn == nil {
			return errs.New(errs.CodeTransactionLookupFailed, "transaction not found for id %v", id)
		}
		logger.Logger(txCtx).Debug("found transaction for delete", "txn", foundTxn.String())

		budget, err := s.budgetRepo.GetById(txCtx, tx, budgetId)
		if err != nil {
			return errs.Wrap(errs.CodeBudgetLookupFailed, "error fetching budget", err)
		}

		if err = s.applySideEffects(txCtx, tx, sideEffectInput{
			budgetId: budgetId,
			oldTxn:   foundTxn,
			newTxn:   nil,
			budget:   budget,
		}); err != nil {
			return err
		}

		if err = s.repo.DeleteById(txCtx, tx, budgetId, id); err != nil {
			return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting transaction", err)
		}

		return nil
	})
}
