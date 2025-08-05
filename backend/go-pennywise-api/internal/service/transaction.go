package service

import (
	"context"
	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type TransactionService interface {
	GetAll(ctx context.Context) ([]model.Transaction, error)
	// GetById(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	// Update(ctx contex.Context, id uuid.UUID, txn model.Transaction) error
	// Create(ctx context.Context, txn model.Transaction) error
	// DeleteById(ctx context.Context, id uuid.UUID) error
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
