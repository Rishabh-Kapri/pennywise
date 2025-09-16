package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

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
	predictionRepo repository.PredictionRepository
	accountRepo    repository.AccountRepository
	payeeRepo      repository.PayeesRepository
	categoryRepo   repository.CategoryRepository
	mbRepo         repository.MonthlyBudgetRepository
}

func NewTransactionService(
	r repository.TransactionRepository,
	predictionRepo repository.PredictionRepository,
	accountRepo repository.AccountRepository,
	payeeRepo repository.PayeesRepository,
	catRepo repository.CategoryRepository,
	mbRepo repository.MonthlyBudgetRepository,
) TransactionService {
	return &transactionService{
		repo:           r,
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
		log.Printf("Prediction not found for txn %v", txnId)
		return nil
	}
	log.Printf("%v", prediction.String())

	log.Printf("%v", txn.String())
	// Use transaction context for all repository calls to avoid deadlocks
	account, err := s.accountRepo.GetByIdTx(ctx, tx, budgetId, *txn.AccountID)
	if err != nil {
		return fmt.Errorf("Error getting account: %v", err)
	}

	payee, err := s.payeeRepo.GetByIdTx(ctx, tx, budgetId, *txn.PayeeID)
	if err != nil {
		return fmt.Errorf("Error getting payee: %v", err)
	}

	category, err := s.categoryRepo.GetByIdSimplifiedTx(ctx, tx, budgetId, *txn.CategoryID)
	if err != nil {
		return fmt.Errorf("Error getting category: %v", err)
	}

	// if the prediction is not the same as the existing one, update it
	trueVal := true
	falseVal := false
	needsUpdate := false

	prediction.HasUserCorrected = &falseVal

	accountName := &account.Name
	payeeName := &payee.Name
	catName := &category.Name

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
	if prediction.Category != nil && *prediction.Category != *catName {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedCategory = &category.Name
		needsUpdate = true
	}

	if needsUpdate {
		log.Printf("Entering PredictionRepo Update for id: %v", txnId)
		err = s.predictionRepo.Update(ctx, tx, budgetId, prediction.ID, *prediction)
		if err != nil {
			return fmt.Errorf("failed to update prediction: %v", err)
		}
	}
	return nil
}

func (s *transactionService) updateCarryovers(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, foundTxn *model.Transaction, txn model.Transaction) error {
	// Handle old category carryover reversal
	existingCatId := foundTxn.CategoryID
	newCatId := txn.CategoryID
	if *existingCatId != uuid.Nil && *existingCatId != *newCatId {
		existingTxnMonthKey := utils.GetMonthKey(foundTxn.Date)
		log.Printf("Updating carryover for old category: %v for month: %v with amount: %v", foundTxn.CategoryID, existingTxnMonthKey, -foundTxn.Amount)
		if err := s.mbRepo.UpdateCarryoverByCatIdAndMonth(
			ctx,
			tx,
			budgetId,
			*foundTxn.CategoryID,
			existingTxnMonthKey,
			-foundTxn.Amount, // reverse the amount
		); err != nil {
			return fmt.Errorf("failed to update old category: %v carryover: %v", *foundTxn.CategoryID, err)
		}
	}

	// Handle new category carryover
	if *newCatId != uuid.Nil {
		if *newCatId == *existingCatId && foundTxn.Amount == txn.Amount {
			log.Printf("Same amount and same category, skipping carryover update")
			return nil
		}
		newTxnMonthKey := utils.GetMonthKey(txn.Date)
		foundMb, err := s.mbRepo.GetByCatIdAndMonth(ctx, tx, budgetId, *txn.CategoryID, newTxnMonthKey)
		log.Printf("Found existing monthly budget: %+v", foundMb)
		if err != nil {
			// if monthly budget doesn't exists, create a new one
			if errors.Is(err, pgx.ErrNoRows) {
				monthlyBudget := model.MonthlyBudget{
					Month:            newTxnMonthKey,
					BudgetID:         budgetId,
					Budgeted:         0.0,
					CarryoverBalance: txn.Amount,
					CategoryID:       *newCatId,
				}
				log.Printf("Creating carryover %+v", monthlyBudget)
				if err = s.mbRepo.Create(ctx, tx, budgetId, monthlyBudget); err != nil {
					return fmt.Errorf("error while creating new monthly budget for category %v and month %v: %w", monthlyBudget.CategoryID, newTxnMonthKey, err)
				}
			} else {
				return fmt.Errorf("error while fetching monthly budget for category %v and month %v: %w", newCatId, newTxnMonthKey, err)
			}
		} else {
			diff := txn.Amount
			if *newCatId == *existingCatId {
				// calculate diff only when updating amount for the same category
				diff = txn.Amount - foundTxn.Amount
			}
			log.Printf("Updating carryover for new category %v for month: %v for amount: %v", txn.CategoryID, newTxnMonthKey, diff)
			if err := s.mbRepo.UpdateCarryoverByCatIdAndMonth(ctx, tx, budgetId, *txn.CategoryID, newTxnMonthKey, diff); err != nil {
				return fmt.Errorf("failed to update new category: %v carryover: %v", *txn.CategoryID, err)
			}
		}
	}

	return nil
}

func (s *transactionService) GetAll(ctx context.Context) ([]model.Transaction, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *transactionService) GetAllNormalized(ctx context.Context, accountId *uuid.UUID) ([]model.Transaction, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAllNormalized(ctx, budgetId, accountId)
}

func (s *transactionService) Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error) {
	tx, err := s.repo.GetPgxTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	txn.BudgetID = budgetId
	createdTxn, err := s.repo.Create(ctx, tx, txn)
	if err != nil {
		return nil, err
	}
	// @TODO: Add better handling for inflow category and other system categories
	if txn.CategoryID != nil && txn.CategoryID.String() != "02fc5abc-94b7-4b03-9077-5d153011fd3f" {
		monthKey := utils.GetMonthKey(txn.Date)
		foundMb, err := s.mbRepo.GetByCatIdAndMonth(ctx, tx, budgetId, *txn.CategoryID, monthKey)
		log.Printf("Found existing monthly budget: %+v", foundMb)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				monthlyBudget := model.MonthlyBudget{
					Month:            monthKey,
					BudgetID:         budgetId,
					Budgeted:         0,
					CarryoverBalance: txn.Amount,
					CategoryID:       *txn.CategoryID,
				}
				if err = s.mbRepo.Create(ctx, tx, budgetId, monthlyBudget); err != nil {
					return nil, fmt.Errorf("error while creating new monthly budget for category %v and month %v: %w", monthlyBudget.CategoryID, monthKey, err)
				}
			} else {
				return nil, fmt.Errorf("error while fetching monthly budget for category %v and month %v: %w", *txn.CategoryID, monthKey, err)
			}
		} else {
			if err = s.mbRepo.UpdateCarryoverByCatIdAndMonth(ctx, tx, budgetId, *txn.CategoryID, monthKey, txn.Amount); err != nil {
				return nil, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return createdTxn, nil
}

func (s *transactionService) Update(ctx context.Context, id uuid.UUID, txn model.Transaction) error {
	// Create a shorter context for each individual transaction attempt
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	log.Printf("UPDATING TXN :%+v", txn)

	tx, err := s.repo.GetPgxTx(txCtx)
	if err != nil {
		return fmt.Errorf("Error getting pgx tx: %v", err)
	}
	defer func() {
		if err = tx.Rollback(txCtx); err != nil {
			log.Printf("WARNING: Error rolling back transaction:  %v", err)
		}
	}()

	foundTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, id)
	if err != nil {
		return fmt.Errorf("Error getting transaction: %v", err)
	}
	if foundTxn == nil {
		return fmt.Errorf("Transaction not found for id %v", id)
	}

	// update prediction if needed
	if foundTxn.Source == "MLP" {
		if err = s.updatePrediction(txCtx, tx, budgetId, id, txn); err != nil {
			return fmt.Errorf("Error updating prediction:  %v", err)
		}
	}

	if err = s.repo.Update(txCtx, tx, budgetId, id, txn); err != nil {
		return fmt.Errorf("Error updating transaction: %v", err)
	}

	if err = s.updateCarryovers(txCtx, tx, budgetId, foundTxn, txn); err != nil {
		return fmt.Errorf("Error updating carryovers: %v", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return fmt.Errorf("Error committing transaction: %v", err)
	}

	return nil
}

func (s *transactionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	log.Printf("DELETING TXN :%v", id)

	tx, err := s.repo.GetPgxTx(txCtx)
	if err != nil {
		return fmt.Errorf("Error getting pgx tx: %v", err)
	}
	defer func() {
		if err = tx.Rollback(txCtx); err != nil {
			log.Printf("WARNING: Error rolling back transaction:  %v", err)
		}
	}()

	foundTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, id)
	if err != nil {
		return fmt.Errorf("Error getting transaction: %v", err)
	}
	if foundTxn == nil {
		return fmt.Errorf("Transaction not found for id %v", id)
	}

	// delete any present prediction
	if foundTxn.Source == "MLP" {
		if err = s.predictionRepo.DeleteByTxnId(txCtx, tx, budgetId, id); err != nil {
			return fmt.Errorf("error while deleting prediction for transaction %v: %w", id, err)
		}
	}

	// reverse carryover
	monthKey := utils.GetMonthKey(foundTxn.Date)
	if err = s.mbRepo.UpdateCarryoverByCatIdAndMonth(txCtx, tx, budgetId, *foundTxn.CategoryID, monthKey, -foundTxn.Amount); err != nil {
		return fmt.Errorf("error while reversing carryover for transaction category %v and month: %w", foundTxn.CategoryID, monthKey, err)
	}

	if err = s.repo.DeleteById(txCtx, tx, budgetId, id); err != nil {
		return fmt.Errorf("error deleting transaction: %v", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("error while commiting delete transaction: %v", err)
	}

	return nil
}
