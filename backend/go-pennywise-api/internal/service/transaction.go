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
}

func NewTransactionService(
	r repository.TransactionRepository,
	predictionRepo repository.PredictionRepository,
	accountRepo repository.AccountRepository,
	payeeRepo repository.PayeesRepository,
	catRepo repository.CategoryRepository,
) TransactionService {
	return &transactionService{
		repo:           r,
		predictionRepo: predictionRepo,
		accountRepo:    accountRepo,
		payeeRepo:      payeeRepo,
		categoryRepo:   catRepo,
	}
}

func (s *transactionService) updatePrediction(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnId uuid.UUID, txn model.Transaction) error {
	prediction, err := s.predictionRepo.GetByTxnIdTx(ctx, tx, budgetId, txnId)
	log.Printf("Prediction:%+v", prediction)
	if err != nil {
		return fmt.Errorf("Error getting prediction: %v", err)
	}
	if prediction == nil {
		log.Printf("Prediction not found for txn %v", txnId)
		return nil
	}

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
	needsUpdate := false

	if prediction.Account != &account.Name {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedAccount = &account.Name
		needsUpdate = true
	}
	if prediction.Payee != &payee.Name {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedPayee = &payee.Name
		needsUpdate = true
	}
	if prediction.Category != &category.Name {
		prediction.HasUserCorrected = &trueVal
		prediction.UserCorrectedCategory = &category.Name
		needsUpdate = true
	}

	if needsUpdate {
		log.Printf("Entering PredictionRepo Update for id: %v", txnId)
		err = s.predictionRepo.Update(ctx, tx, budgetId, prediction.ID, *prediction)
		log.Printf("Exited PredictionRepo Update for id: %v, err: %v", txnId, err)
		if err != nil {
			return fmt.Errorf("failed to update prediction: %v", err)
		}
	}
	return nil
}

func (s *transactionService) updateCarryovers(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, foundTxn *model.Transaction, txn model.Transaction, monthKey string) error {
	// Handle old category carryover reversal
	if foundTxn.CategoryID != txn.CategoryID && foundTxn.CategoryID != nil {
		log.Printf("Updating carryover for old category: %v for month: %v", foundTxn.CategoryID, monthKey)
		if err := utils.UpdateCarryover(ctx, tx, budgetId, *foundTxn.CategoryID, -foundTxn.Amount, monthKey); err != nil {
			return fmt.Errorf("failed to update old category carryover: %v", err)
		}
	}

	// Handle new category carryover
	if txn.CategoryID != nil {
		log.Printf("Updating carryover for new category %v for month: %v", txn.CategoryID, monthKey)
		if err := utils.UpdateCarryover(ctx, tx, budgetId, *txn.CategoryID, txn.Amount, monthKey); err != nil {
			return fmt.Errorf("failed to update new category carryover: %v", err)
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
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	txn.BudgetID = budgetId
	return s.repo.Create(ctx, txn)
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
		log.Printf("rollbacking transaction: %v", err)
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
	log.Printf("FOUND TXN: %+v", foundTxn)

	// update prediction if needed
	if foundTxn.Source == "MLP" {
		log.Printf("Updating prediction...")
		if err = s.updatePrediction(txCtx, tx, budgetId, id, txn); err != nil {
			log.Printf("ERROR: Prediction update failed: %v", err)
			return err
		}
		log.Printf("SUCCESS: Prediction updated")
	}

	log.Printf("=== CALLING TRANSACTION REPO UPDATE ===")
	start := time.Now()
	if err = s.repo.Update(txCtx, tx, budgetId, id, txn); err != nil {
		duration := time.Since(start)
		log.Printf("ERROR: Transaction update failed after %v: %v", duration, err)
		return fmt.Errorf("Error updating transaction: %v", err)
	}
	duration := time.Since(start)
	log.Printf("SUCCESS: Transaction update completed in %v", duration)

	monthKey := utils.GetMonthKey(txn.Date)
	log.Printf("Month key: %s", monthKey)

	log.Printf("Updating carryovers...")
	if err = s.updateCarryovers(txCtx, tx, budgetId, foundTxn, txn, monthKey); err != nil {
		log.Printf("ERROR: Carryover update failed: %v", err)
		return err
	}
	log.Printf("SUCCESS: Carryovers updated")

	log.Printf("Committing transaction...")
	if err := tx.Commit(txCtx); err != nil {
		log.Printf("ERROR: Commit failed: %v", err)
		return err
	}
	log.Printf("SUCCESS: Transaction committed")
	log.Printf("=== UPDATE COMPLETED for txn ID: %v ===", id)

	return nil
}

func (s *transactionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.DeleteById(ctx, budgetId, id)
}
