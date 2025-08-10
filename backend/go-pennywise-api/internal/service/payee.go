package service

import (
	"context"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type PayeeService interface {
	GetAll(ctx context.Context) ([]model.Payee, error)
	Search(ctx context.Context, query string) ([]model.Payee, error)
	GetById(ctx context.Context, id uuid.UUID) (*model.Payee, error)
	Create(ctx context.Context, payee model.Payee) error
	DeleteById(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, payee model.Payee) error
}

type payeeService struct {
	repo repository.PayeesRepository
}

func NewPayeeService(repo repository.PayeesRepository) PayeeService {
	return &payeeService{repo}
}

func (s *payeeService) GetAll(ctx context.Context) ([]model.Payee, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *payeeService) Search(ctx context.Context, query string) ([]model.Payee, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Search(ctx, budgetId, query)
}

func (s *payeeService) GetById(ctx context.Context, id uuid.UUID) (*model.Payee, error) {
	budgetId := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetById(ctx, budgetId, id)
}

func (s *payeeService) Create(ctx context.Context, payee model.Payee) error {
	budgetId := ctx.Value("budgetId").(uuid.UUID)
	payee.BudgetID = budgetId
	return s.repo.Create(ctx, payee)
}

func (s *payeeService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.DeleteById(ctx, budgetId, id)
}

func (s *payeeService) Update(ctx context.Context, id uuid.UUID, payee model.Payee) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Update(ctx, budgetId, id, payee)
}
