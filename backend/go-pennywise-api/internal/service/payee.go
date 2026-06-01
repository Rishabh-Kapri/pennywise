package service

import (
	"context"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PayeeService interface {
	GetAll(ctx context.Context) ([]model.Payee, error)
	Search(ctx context.Context, query string) ([]model.Payee, error)
	GetById(ctx context.Context, id uuid.UUID) (*model.Payee, error)
	GetRules(ctx context.Context, id uuid.UUID) ([]model.PayeeRuleDetails, error)
	CreateRule(ctx context.Context, id uuid.UUID, payeeRule model.PayeeRule) error
	UpdateRule(ctx context.Context, id uuid.UUID, ruleId uuid.UUID, payeeRule model.PayeeRule) error
	DeleteRule(ctx context.Context, ruleId uuid.UUID) error
	Create(ctx context.Context, payee model.Payee) (*model.Payee, error)
	CreateWithTx(ctx context.Context, tx pgx.Tx, payee model.Payee) (*model.Payee, error)
	DeleteById(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, payee model.Payee) error
}

type payeeService struct {
	repo          repository.PayeesRepository
	payeeRuleRepo repository.PayeeRuleRepository
}

func NewPayeeService(repo repository.PayeesRepository, payeeRuleRepo repository.PayeeRuleRepository) PayeeService {
	return &payeeService{repo: repo, payeeRuleRepo: payeeRuleRepo}
}

func (s *payeeService) GetAll(ctx context.Context) ([]model.Payee, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *payeeService) Search(ctx context.Context, query string) ([]model.Payee, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Search(ctx, budgetId, query)
}

func (s *payeeService) GetById(ctx context.Context, id uuid.UUID) (*model.Payee, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetById(ctx, budgetId, id)
}

func (s *payeeService) GetRules(ctx context.Context, id uuid.UUID) ([]model.PayeeRuleDetails, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.payeeRuleRepo.FindByPayeeID(ctx, budgetId, id)
}

func (s *payeeService) CreateRule(ctx context.Context, id uuid.UUID, payeeRule model.PayeeRule) error {
	budgetId := utils.MustBudgetID(ctx)
	payeeRule.BudgetID = budgetId
	payeeRule.PayeeID = id
	if payeeRule.MatchType == "" {
		payeeRule.MatchType = "EXACT"
	}
	return s.payeeRuleRepo.CreatePayeeRule(ctx, nil, payeeRule)
}

func (s *payeeService) UpdateRule(ctx context.Context, id uuid.UUID, ruleId uuid.UUID, payeeRule model.PayeeRule) error {
	budgetId := utils.MustBudgetID(ctx)
	payeeRule.BudgetID = budgetId
	payeeRule.PayeeID = id
	if payeeRule.MatchType == "" {
		payeeRule.MatchType = "EXACT"
	}
	return s.payeeRuleRepo.Update(ctx, budgetId, ruleId, payeeRule)
}

func (s *payeeService) DeleteRule(ctx context.Context, ruleId uuid.UUID) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.payeeRuleRepo.DeleteByID(ctx, budgetId, ruleId)
}

func (s *payeeService) Create(ctx context.Context, payee model.Payee) (*model.Payee, error) {
	budgetId := utils.MustBudgetID(ctx)
	payee.BudgetID = budgetId
	return s.CreateWithTx(ctx, nil, payee)
}

func (s *payeeService) CreateWithTx(ctx context.Context, tx pgx.Tx, payee model.Payee) (*model.Payee, error) {
	budgetId := utils.MustBudgetID(ctx)
	payee.BudgetID = budgetId
	return s.repo.Create(ctx, tx, payee)
}

func (s *payeeService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.DeleteById(ctx, budgetId, id)
}

func (s *payeeService) Update(ctx context.Context, id uuid.UUID, payee model.Payee) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Update(ctx, budgetId, id, payee)
}
