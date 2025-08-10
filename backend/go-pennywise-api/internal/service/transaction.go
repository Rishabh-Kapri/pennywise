package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type TransactionService interface {
	GetAll(ctx context.Context) ([]model.Transaction, error)
	GetAllNormalized(ctx context.Context) ([]model.Transaction, error)
	// GetById(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	Update(ctx context.Context, id uuid.UUID, txn model.Transaction) error
	Create(ctx context.Context, txn model.Transaction) (*model.Transaction, error)
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type transactionService struct {
	repo repository.TransactionRepository
}

func NewTransactionService(r repository.TransactionRepository) TransactionService {
	return &transactionService{repo: r}
}

func (s *transactionService) GetAll(ctx context.Context) ([]model.Transaction, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *transactionService) GetAllNormalized(ctx context.Context) ([]model.Transaction, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAllNormalized(ctx, budgetId)
}

func (s *transactionService) Create(ctx context.Context, txn model.Transaction) (*model.Transaction, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	txn.BudgetID = budgetId
	return s.repo.Create(ctx, txn)
}

func (s *transactionService) Update(ctx context.Context, id uuid.UUID, txn model.Transaction) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Update(ctx, budgetId, id, txn)
}

func (s *transactionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.DeleteById(ctx, budgetId, id)
}
