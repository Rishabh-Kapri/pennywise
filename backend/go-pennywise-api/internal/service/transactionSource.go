package service

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// TransactionSourceService serves the bank text behind a transaction.
type TransactionSourceService interface {
	// ListByTransaction returns the transaction's sources, oldest first.
	ListByTransaction(ctx context.Context, transactionID uuid.UUID) ([]model.TransactionSource, error)
}

type transactionSourceService struct {
	repo repository.TransactionSourceRepository
}

func NewTransactionSourceService(repo repository.TransactionSourceRepository) TransactionSourceService {
	return &transactionSourceService{repo: repo}
}

func (s *transactionSourceService) ListByTransaction(
	ctx context.Context,
	transactionID uuid.UUID,
) ([]model.TransactionSource, error) {
	return s.repo.ListByTransaction(ctx, utils.MustBudgetID(ctx), transactionID)
}
