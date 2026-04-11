package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	errs "pennywise-api/internal/errors"
	utils "pennywise-api/pkg"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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
	mbRepo         repository.MonthlyBudgetRepository
}

type txnDiff struct {
	oldCatId    *uuid.UUID
	newCatId    *uuid.UUID
	oldMonthKey string
	newMonthKey string
	oldAmount   float64
	newAmount   float64
}

// carryoverCase is a helper struct for carryover logic
type carryoverCase struct {
	sameCategory bool
	sameMonth    bool
}

type carryoverOp struct {
	categoryId  uuid.UUID
	monthKey    string
	amountDelta float64
}

func NewTransactionService(
	r repository.TransactionRepository,
	budgetRepo repository.BudgetRepository,
	predictionRepo repository.PredictionRepository,
	accountRepo repository.AccountRepository,
	payeeRepo repository.PayeesRepository,
	catRepo repository.CategoryRepository,
	mbRepo repository.MonthlyBudgetRepository,
) TransactionService {
	return &transactionService{
		repo:           r,
		budgetRepo:     budgetRepo,
		predictionRepo: predictionRepo,
		accountRepo:    accountRepo,
		payeeRepo:      payeeRepo,
		categoryRepo:   catRepo,
		mbRepo:         mbRepo,
	}
}

// internal method
func (s *transactionService) updatePrediction(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnId uuid.UUID, txn model.Transaction) error {
	prediction, err := s.predictionRepo.GetByTxnIdTx(ctx, tx, budgetId, txnId)
	if err != nil {
		return fmt.Errorf("Error getting prediction: %v", err)
	}
	if prediction == nil {
		utils.Logger(ctx).Info("prediction not found for txn", "txnId", txnId)
		return nil
	}
	utils.Logger(ctx).Debug("prediction", "prediction", prediction.String())

	utils.Logger(ctx).Debug("transaction", "txn", txn.String())
	// Use transaction context for all repository calls to avoid deadlocks
	account, err := s.accountRepo.GetById(ctx, tx, budgetId, *txn.AccountID)
	if err != nil {
		return fmt.Errorf("Error getting account: %v", err)
	}

	payee, err := s.payeeRepo.GetByIdTx(ctx, tx, budgetId, *txn.PayeeID)
	if err != nil {
		return fmt.Errorf("Error getting payee: %v", err)
	}

	var category *model.Category
	if txn.CategoryID != nil {
		category, err = s.categoryRepo.GetByIdSimplifiedTx(ctx, tx, budgetId, *txn.CategoryID)
		if err != nil {
			return fmt.Errorf("Error getting category: %v", err)
		}
	}

	// if the prediction is not the same as the existing one, update it
	trueVal := true
	falseVal := false
	needsUpdate := false

	prediction.HasUserCorrected = &falseVal

	accountName := &account.Name
	payeeName := &payee.Name
	var catName *string
	if category != nil {
		catName = &category.Name
	}

	if prediction.Account != nil && *prediction.Account != *accountName {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedAccount = &account.Name
		needsUpdate = true
	}
	if prediction.Payee != nil && *prediction.Payee != *payeeName {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedPayee = &payee.Name
		needsUpdate = true
	}
	if prediction.Category != nil && catName != nil && *prediction.Category != *catName {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedCategory = &category.Name
		needsUpdate = true
	}

	if needsUpdate {
		utils.Logger(ctx).Info("updating prediction", "txnId", txnId)
		err = s.predictionRepo.Update(ctx, tx, budgetId, prediction.ID, *prediction)
		if err != nil {
			return fmt.Errorf("failed to update prediction: %v", err)
		}
	}
	return nil
}

// returns a list of carryover operations to be performed
func (s *transactionService) getCarryoverOps(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnDiff *txnDiff, carryoverCase carryoverCase) []carryoverOp {
	var carryoverOps []carryoverOp
	diff := txnDiff.newAmount - txnDiff.oldAmount

	switch {
	// same category and month
	case carryoverCase.sameCategory && carryoverCase.sameMonth:
		{
			// check if amount has changed
			if diff == 0 {
				return carryoverOps
			}
			// amount has changed
			carryoverOps = append(carryoverOps, carryoverOp{
				categoryId:  *txnDiff.newCatId,   // categoryId is the same as oldCatId
				monthKey:    txnDiff.newMonthKey, // monthKey is the same as oldMonthKey
				amountDelta: diff,
			})
			return carryoverOps
		}
	// same category and different month
	case carryoverCase.sameCategory && !carryoverCase.sameMonth:
		// catA, 2025-11, amount is 100,
		// txn updated to 2025-12 with amount of 150
		// catA, 2025-12, amount is 100
		{
			carryoverOps = append(carryoverOps, carryoverOp{
				categoryId:  *txnDiff.oldCatId, // categoryId is the same as oldCatId
				monthKey:    txnDiff.oldMonthKey,
				amountDelta: -txnDiff.oldAmount, // reverse the amount from old month
			})
			carryoverOps = append(carryoverOps, carryoverOp{
				categoryId:  *txnDiff.newCatId,
				monthKey:    txnDiff.newMonthKey,
				amountDelta: txnDiff.newAmount,
			})
			return carryoverOps
		}
	// different category and same month
	case !carryoverCase.sameCategory && carryoverCase.sameMonth:
		{
			carryoverOps = append(carryoverOps, carryoverOp{
				categoryId:  *txnDiff.oldCatId,
				monthKey:    txnDiff.oldMonthKey,
				amountDelta: -txnDiff.oldAmount,
			})
			carryoverOps = append(carryoverOps, carryoverOp{
				categoryId:  *txnDiff.newCatId,
				monthKey:    txnDiff.newMonthKey,
				amountDelta: txnDiff.newAmount,
			})
			return carryoverOps
		}
	// different category and different month
	case !carryoverCase.sameCategory && !carryoverCase.sameMonth:
		{
			carryoverOps = append(carryoverOps, carryoverOp{
				categoryId:  *txnDiff.oldCatId,
				monthKey:    txnDiff.oldMonthKey,
				amountDelta: -txnDiff.oldAmount,
			})
			carryoverOps = append(carryoverOps, carryoverOp{
				categoryId:  *txnDiff.newCatId,
				monthKey:    txnDiff.newMonthKey,
				amountDelta: txnDiff.newAmount,
			})
			return carryoverOps
		}
	}

	return carryoverOps
}

func (s *transactionService) applyCarryoverOps(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnDiff *txnDiff, carryoverCase carryoverCase) error {
	carryoverOps := s.getCarryoverOps(ctx, tx, budgetId, txnDiff, carryoverCase)
	for _, op := range carryoverOps {
		_, err := s.mbRepo.GetByCatIdAndMonth(ctx, tx, budgetId, op.categoryId, op.monthKey)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// if monthly budget doesn't exists, create a new one
				monthlyBudget := model.MonthlyBudget{
					Month:            op.monthKey,
					BudgetID:         budgetId,
					Budgeted:         0,
					CarryoverBalance: op.amountDelta,
					CategoryID:       op.categoryId,
				}
				if err = s.mbRepo.Create(ctx, tx, budgetId, monthlyBudget); err != nil {
					return errs.Wrap(errs.CodeMonthlyBudgetCreateFailed, "error while creating monthly budget", err)
				}
			} else {
				return errs.Wrap(errs.CodeMonthlyBudgetLookupFailed, "error while fetching monthly budget", err)
			}
		} else {
			// monthly budget exists, update carryover
			if err = s.mbRepo.UpdateCarryoverByCatIdAndMonth(ctx, tx, budgetId, op.categoryId, op.monthKey, op.amountDelta); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateTransactionPayload validates the payload of a transaction
func (s *transactionService) validateTransactionPayload(txn model.Transaction) error {
	if txn.AccountID == nil {
		return errs.New(errs.CodeInvalidArgument, "account_id is required")
	}
	if txn.PayeeID == nil {
		return errs.New(errs.CodeInvalidArgument, "payee_id is required")
	}
	if err := txn.Date.Valid(); err != nil {
		return err
	}
	if txn.Amount <= 0 {
		return errs.New(errs.CodeInvalidArgument, "amount must be greater than 0")
	}

	return nil
}

// loadDependencies loads the budget, account, and payee for the transaction
func (s *transactionService) loadDependencies(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txn model.Transaction) (*model.Budget, *model.Account, *model.Payee, error) {
	budget, err := s.budgetRepo.GetById(ctx, tx, budgetId)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodeBudgetLookupFailed, "error while fetching budget with id %v", err)
	}

	account, err := s.accountRepo.GetById(ctx, tx, budgetId, *txn.AccountID)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodeAccountLookupFailed, "error getting account", err)
	}

	payee, err := s.payeeRepo.GetByIdTx(ctx, tx, budgetId, *txn.PayeeID)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodePayeeLookupFailed, "error getting payee", err)
	}

	return budget, account, payee, nil
}

// validate the category of the transaction
// for budget transfers, the category should be nil
func (s *transactionService) validateCategory(categoryID *uuid.UUID, account model.Account, payee model.Payee) error {
	// budget -> budget transfers don't have a category
	if payee.TransferAccountID != nil {
		if account.Type == "savings" || account.Type == "checking" || account.Type == "creditCard" {
			if categoryID != nil {
				return errs.New(errs.CodeInvalidArgument, "category is not allowed for budget transfers")
			}
		}
	}
	return nil
}

func (s *transactionService) createTransferTxnIfNeeded(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnPayload model.Transaction, account model.Account, payee model.Payee) (*uuid.UUID, error) {
	if payee.TransferAccountID != nil {
		// this is a transfer transaction
		transferTxn := model.Transaction{
			BudgetID:              budgetId,
			AccountID:             payee.TransferAccountID,
			PayeeID:               account.TransferPayeeID,
			CategoryID:            nil,
			Amount:                -txnPayload.Amount,
			Date:                  txnPayload.Date,
			Note:                  txnPayload.Note,
			Source:                txnPayload.Source,
			TransferAccountID:     txnPayload.AccountID,
			TransferTransactionID: &txnPayload.ID,
		}
		createdTransferTxn, err := s.repo.Create(ctx, tx, transferTxn)
		if err != nil {
			return nil, errs.Wrap(errs.CodeTransferCreateFailed, "error while creating transfer transaction", err)
		}
		if len(createdTransferTxn) == 0 {
			return nil, errs.New(errs.CodeTransferNotCreated, "no transfer transaction was created")
		}
		transferTxnId := createdTransferTxn[0].ID
		txnPayload.TransferTransactionID = &transferTxnId
		err = s.repo.Update(ctx, tx, budgetId, txnPayload.ID, txnPayload)
		if err != nil {
			return nil, errs.Wrap(errs.CodeTransactionUpdateFailed, "error while updating transaction", err)
		}
		return &createdTransferTxn[0].ID, nil
	}

	// not a transfer transaction
	return nil, nil
}

// applySideEffects applies any side effects to the transaction
// such as carryovers, predictions, etc.
func (s *transactionService) applySideEffects(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txn model.Transaction, budget model.Budget) error {
	if txn.CategoryID != nil && txn.CategoryID.String() != budget.Metadata.InflowCategoryID.String() {
		monthKey := utils.GetMonthKey(txn.Date.String())
		foundMb, err := s.mbRepo.GetByCatIdAndMonth(ctx, tx, budgetId, *txn.CategoryID, monthKey)

		utils.Logger(ctx).Debug("found existing monthly budget", "monthlyBudget", foundMb)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// if monthly budget doesn't exists, create a new one
				monthlyBudget := model.MonthlyBudget{
					Month:            monthKey,
					BudgetID:         budgetId,
					Budgeted:         0,
					CarryoverBalance: txn.Amount,
					CategoryID:       *txn.CategoryID,
				}
				if err = s.mbRepo.Create(ctx, tx, budgetId, monthlyBudget); err != nil {
					return errs.Wrap(errs.CodeMonthlyBudgetCreateFailed, "error while creating monthly budget", err)
				}
			} else {
				return errs.Wrap(errs.CodeMonthlyBudgetLookupFailed, "error while fetching monthly budget", err)
			}
		} else {
			// monthly budget exists, update carryover
			if err = s.mbRepo.UpdateCarryoverByCatIdAndMonth(ctx, tx, budgetId, *txn.CategoryID, monthKey, txn.Amount); err != nil {
				return err
			}
		}
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

	budgetId := utils.MustBudgetID(ctx)
	txn.BudgetID = budgetId

	if err := s.validateTransactionPayload(txn); err != nil {
		return nil, err
	}

	var createdTxn []model.Transaction
	err := utils.WithTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		var err error

		budget, account, payee, err := s.loadDependencies(ctx, tx, budgetId, txn)
		if err != nil {
			return err
		}

		if err = s.validateCategory(txn.CategoryID, *account, *payee); err != nil {
			return err
		}

		// clear transfer fields in case they are set
		txn.TransferAccountID = nil
		txn.TransferTransactionID = nil

		createdTxn, err = s.repo.Create(ctx, tx, txn)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionCreateFailed, "failed to create transaction", err)
		}
		if len(createdTxn) == 0 {
			return errs.New(errs.CodeTransactionNotCreated, "no transaction was created")
		}

		txn.ID = createdTxn[0].ID

		// create transfer transaction if needed
		_, err = s.createTransferTxnIfNeeded(ctx, tx, budgetId, txn, *account, *payee)
		if err != nil {
			return err
		}

		if err = s.applySideEffects(ctx, tx, budgetId, txn, *budget); err != nil {
			return err
		}

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
	utils.Logger(ctx).Info("updating transaction", "id", id)

	if err := s.validateTransactionPayload(txn); err != nil {
		return err
	}

	return utils.WithTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		foundTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, id)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting transaction", err)
		}
		if foundTxn == nil {
			return errs.New(errs.CodeTransactionLookupFailed, "transaction not found for id %v", id)
		}

		same := foundTxn.Compare(&txn)
		if same {
			utils.Logger(txCtx).Info("transaction is the same as the existing transaction, skipping update")
			return nil
		}

		// fetch existing txn account and payee
		// _, existingAccount, existingPayee, err := s.loadDependencies(txCtx, tx, budgetId, foundTxn)
		// if err != nil {
		// 	return err
		// }

		// fetch updated txn account and payee
		_, account, payee, err := s.loadDependencies(txCtx, tx, budgetId, txn)
		if err != nil {
			return err
		}

		err = s.validateCategory(txn.CategoryID, *account, *payee)
		if err != nil {
			return err
		}

		txnDiff := &txnDiff{
			oldCatId:    foundTxn.CategoryID,
			newCatId:    txn.CategoryID,
			oldMonthKey: utils.GetMonthKey(foundTxn.Date.String()),
			newMonthKey: utils.GetMonthKey(txn.Date.String()),
			oldAmount:   foundTxn.Amount,
			newAmount:   txn.Amount,
		}
		carryoverCase := carryoverCase{
			sameCategory: foundTxn.CategoryID != nil && txn.CategoryID != nil && *foundTxn.CategoryID == *txn.CategoryID,
			sameMonth:    foundTxn.Date.String() == txn.Date.String(),
		}
		err = s.applyCarryoverOps(ctx, tx, budgetId, txnDiff, carryoverCase)
		if err != nil {
			return err
		}

		// transfer txn cases
		// case 1: creating a new transfer transaction
		//    -
		// case 2: updating an existing transfer transaction
		// case 3: converting a transfer transaction to a regular transaction

		switch {
		case foundTxn.TransferTransactionID == nil && payee.TransferAccountID != nil:
			{
				// case 1: creating a new transfer transaction
				utils.Logger(txCtx).Info("creating new transfer transaction")

				// we are creating a new transfer transaction
				foundTxnId := foundTxn.ID
				transferTxn := model.Transaction{
					BudgetID:              budgetId,
					AccountID:             payee.TransferAccountID,
					PayeeID:               account.TransferPayeeID,
					CategoryID:            nil,
					Amount:                -txn.Amount,
					Date:                  txn.Date,
					Note:                  txn.Note,
					Source:                txn.Source,
					TransferAccountID:     txn.AccountID,
					TransferTransactionID: &foundTxnId,
				}
				createdTransferTxn, err := s.repo.Create(txCtx, tx, transferTxn)
				if err != nil {
					return fmt.Errorf("error while creating transfer transaction: %v", err)
				}
				if len(createdTransferTxn) == 0 {
					return fmt.Errorf("no transfer transaction was created")
				}
				createdTransferTxnId := createdTransferTxn[0].ID
				// update existing transaction
				updateTxn := model.Transaction{
					BudgetID:              budgetId,
					AccountID:             txn.AccountID,
					PayeeID:               txn.PayeeID,
					CategoryID:            txn.CategoryID,
					Amount:                txn.Amount,
					Date:                  txn.Date,
					Note:                  txn.Note,
					Source:                txn.Source,
					TransferAccountID:     transferTxn.AccountID,
					TransferTransactionID: &createdTransferTxnId,
				}
				err = s.repo.Update(txCtx, tx, budgetId, foundTxnId, updateTxn)
				if err != nil {
					return errs.Wrap(errs.CodeTransactionUpdateFailed, fmt.Sprintf("error while updating TransferTransactionID for transaction %v", foundTxnId), err)
				}
			}
		case foundTxn.TransferTransactionID != nil:
			{
				// case 2: updating an existing transfer transaction
				utils.Logger(txCtx).Info("updating existing transfer transaction", "transferTxnId", foundTxn.TransferTransactionID)

				// fetch existing transfer transaction
				existingTransferTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, *foundTxn.TransferTransactionID)
				if err != nil {
					return errs.Wrap(errs.CodeTransactionLookupFailed, fmt.Sprintf("error fetching existing transfer transaction %v", *foundTxn.TransferTransactionID), err)
				}
				if existingTransferTxn == nil {
					return errs.New(errs.CodeTransactionLookupFailed, fmt.Sprintf("transfer transaction %v not found", *foundTxn.TransferTransactionID))
				}
				// check if the payee or account has changed
				if existingTransferTxn.PayeeID != account.TransferPayeeID || existingTransferTxn.AccountID != payee.TransferAccountID || existingTransferTxn.Amount != -txn.Amount {
					// update the transfer transaction to reflect changes
					updatedTransferTxn := model.Transaction{
						BudgetID:              budgetId,
						AccountID:             payee.TransferAccountID,
						PayeeID:               account.TransferPayeeID,
						CategoryID:            nil,
						Amount:                -txn.Amount,
						Date:                  txn.Date,
						Note:                  txn.Note,
						Source:                txn.Source,
						TransferAccountID:     txn.AccountID,
						TransferTransactionID: &foundTxn.ID,
					}
					err = s.repo.Update(txCtx, tx, budgetId, *foundTxn.TransferTransactionID, updatedTransferTxn)
					if err != nil {
						return errs.Wrap(errs.CodeTransactionUpdateFailed, fmt.Sprintf("error while updating TransferTransactionID for transaction %v", foundTxn.ID), err)
					}
				}
			}
		}

		// check if
		hasUpdated := false

		// update prediction if needed
		if foundTxn.Source == "MLP" {
			if err = s.updatePrediction(txCtx, tx, budgetId, id, txn); err != nil {
				return fmt.Errorf("error updating prediction:  %v", err)
			}
		}

		if !hasUpdated {
			if err = s.repo.Update(txCtx, tx, budgetId, id, txn); err != nil {
				return fmt.Errorf("error updating transaction: %v", err)
			}
		}

		return nil
	})
}

func (s *transactionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetId := utils.MustBudgetID(ctx)
	utils.Logger(ctx).Info("deleting transaction", "id", id)

	return utils.WithTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		foundTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, id)
		utils.Logger(txCtx).Debug("found transaction for delete", "txn", foundTxn.String())

		if err != nil {
			return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting transaction", err)
		}
		if foundTxn == nil {
			return errs.New(errs.CodeTransactionLookupFailed, "transaction not found for id %v", id)
		}

		if foundTxn.TransferTransactionID != nil {
			// delete transfer txn
			if err = s.repo.DeleteById(txCtx, tx, budgetId, *foundTxn.TransferTransactionID); err != nil {
				return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting transfer transaction", err)
			}
			utils.Logger(txCtx).Debug("deleted transfer transaction", "transferTxnId", *foundTxn.TransferTransactionID)
		}

		// delete any present prediction
		foundPrediction, err := s.predictionRepo.GetByTxnIdTx(txCtx, tx, budgetId, id)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting prediction", err)
		}
		utils.Logger(txCtx).Debug("found prediction", "prediction", foundPrediction.String())

		if foundPrediction != nil {
			if err = s.predictionRepo.DeleteByTxnId(txCtx, tx, budgetId, id); err != nil {
				return errs.Wrap(errs.CodeTransactionDeleteFailed, "error while deleting prediction for transaction %v", err)
			}
			utils.Logger(txCtx).Debug("deleted prediction", "prediction", foundPrediction.ID.String())
		}

		// reverse carryover
		monthKey := utils.GetMonthKey(foundTxn.Date.String())

		utils.Logger(txCtx).Debug("reversing carryover for delete", "txn", foundTxn.String())

		if foundTxn.CategoryID != nil {
			if err = s.mbRepo.UpdateCarryoverByCatIdAndMonth(txCtx, tx, budgetId, *foundTxn.CategoryID, monthKey, -foundTxn.Amount); err != nil {
				return errs.Wrap(errs.CodeTransactionDeleteFailed, "error while reversing carryover for transaction category %v and month %v", err)
			}
		}

		if err = s.repo.DeleteById(txCtx, tx, budgetId, id); err != nil {
			return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting transaction", err)
		}

		return nil
	})
}
