package service

import (
	"context"
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
	log.Printf("%v, %v", prediction, err)
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

	if *prediction.Account != *accountName {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedAccount = &account.Name
		needsUpdate = true
	}
	if *prediction.Payee != *payeeName {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedPayee = &payee.Name
		needsUpdate = true
	}
	if *prediction.Category != *catName {
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
	if foundTxn.CategoryID != nil && foundTxn.CategoryID != txn.CategoryID {
		existingTxnMonthKey := utils.GetMonthKey(foundTxn.Date)
		log.Printf("Updating carryover for old category: %v for month: %v with amount: %v", foundTxn.CategoryID, existingTxnMonthKey, -foundTxn.Amount)
		if err := s.mbRepo.UpdateCarryoverByCatIdAndMonth(ctx, tx, budgetId, *foundTxn.CategoryID, existingTxnMonthKey, -foundTxn.Amount); err != nil {
			return fmt.Errorf("failed to update old category: %v carryover: %v", *foundTxn.CategoryID, err)
		}
	}

	// Handle new category carryover
	if txn.CategoryID != nil {
		newTxnMonthKey := utils.GetMonthKey(txn.Date)
		log.Printf("Updating carryover for new category %v for month: %v", txn.CategoryID, newTxnMonthKey)
		if err := s.mbRepo.UpdateCarryoverByCatIdAndMonth(ctx, tx, budgetId, *txn.CategoryID, newTxnMonthKey, txn.Amount); err != nil {
			return fmt.Errorf("failed to update new category: %v carryover: %v", *txn.CategoryID, err)
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
		if err = s.mbRepo.UpdateCarryoverByCatIdAndMonth(ctx, tx, budgetId, *txn.CategoryID, monthKey, txn.Amount); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return createdTxn, nil
}

func (s *transactionService) Update(ctx context.Context, id uuid.UUID, txn model.Transaction) error {
	// Create a shorter context for each individual transaction attempt
	txCtx, txCancel := context.WithTimeout(ctx, 5*time.Second)
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
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.DeleteById(ctx, budgetId, id)
}
