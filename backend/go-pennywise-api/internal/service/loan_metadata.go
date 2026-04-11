package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"
	utils "pennywise-api/pkg"

	"github.com/google/uuid"
)

type LoanMetadataService interface {
	GetAll(ctx context.Context) ([]model.LoanMetadata, error)
	GetByAccountId(ctx context.Context, accountId uuid.UUID) (*model.LoanMetadata, error)
	Create(ctx context.Context, loan model.LoanMetadata) (*model.LoanMetadata, error)
	Update(ctx context.Context, accountId uuid.UUID, loan model.LoanMetadata) (*model.LoanMetadata, error)
	Delete(ctx context.Context, accountId uuid.UUID) error
}

type loanMetadataService struct {
	repo repository.LoanMetadataRepository
}

func NewLoanMetadataService(r repository.LoanMetadataRepository) LoanMetadataService {
	return &loanMetadataService{repo: r}
}

func (s *loanMetadataService) GetAll(ctx context.Context) ([]model.LoanMetadata, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAllByBudgetId(ctx, budgetId)
}

func (s *loanMetadataService) GetByAccountId(ctx context.Context, accountId uuid.UUID) (*model.LoanMetadata, error) {
	return s.repo.GetByAccountId(ctx, accountId)
}

func (s *loanMetadataService) Create(ctx context.Context, loan model.LoanMetadata) (*model.LoanMetadata, error) {
	return s.repo.Create(ctx, loan)
}

func (s *loanMetadataService) Update(ctx context.Context, accountId uuid.UUID, loan model.LoanMetadata) (*model.LoanMetadata, error) {
	return s.repo.Update(ctx, accountId, loan)
}

func (s *loanMetadataService) Delete(ctx context.Context, accountId uuid.UUID) error {
	return s.repo.Delete(ctx, accountId)
}
