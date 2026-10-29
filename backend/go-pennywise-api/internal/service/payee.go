package service

import (
	"context"
	"errors"
	"fmt"

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
	GetStats(ctx context.Context) ([]model.PayeeStats, error)
	GetAllRules(ctx context.Context) ([]model.PayeeRuleDetails, error)
	Merge(ctx context.Context, req model.PayeeMergeRequest) (*model.PayeeMergeResult, error)
}

var (
	ErrPayeeNotFound     = errors.New("payee not found")
	ErrTransferPayee     = errors.New("transfer payees belong to an account and can't be merged or deleted")
	ErrInvalidPayeeMerge = errors.New("pick a target and at least one other payee to merge")
)

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
	payee, err := s.repo.GetById(ctx, budgetId, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPayeeNotFound
		}
		return err
	}
	if payee.TransferAccountID != nil {
		return ErrTransferPayee
	}
	return s.repo.DeleteById(ctx, budgetId, id)
}

func (s *payeeService) Update(ctx context.Context, id uuid.UUID, payee model.Payee) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Update(ctx, budgetId, id, payee)
}

func (s *payeeService) GetStats(ctx context.Context) ([]model.PayeeStats, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetStats(ctx, budgetId)
}

func (s *payeeService) GetAllRules(ctx context.Context) ([]model.PayeeRuleDetails, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.payeeRuleRepo.GetAll(ctx, budgetId)
}

// Merge folds the source payees into the target in one transaction:
// transactions, rules, recurring schedules, learned embeddings and prediction
// history move over, and the sources are deleted.
func (s *payeeService) Merge(ctx context.Context, req model.PayeeMergeRequest) (*model.PayeeMergeResult, error) {
	budgetId := utils.MustBudgetID(ctx)

	seen := map[uuid.UUID]bool{req.TargetID: true}
	sourceIds := make([]uuid.UUID, 0, len(req.SourceIDs))
	for _, id := range req.SourceIDs {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		sourceIds = append(sourceIds, id)
	}
	if req.TargetID == uuid.Nil || len(sourceIds) == 0 {
		return nil, ErrInvalidPayeeMerge
	}

	var result *model.PayeeMergeResult
	err := utils.WithTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		for _, id := range append([]uuid.UUID{req.TargetID}, sourceIds...) {
			payee, err := s.repo.GetByIdTx(ctx, tx, budgetId, id)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: %s", ErrPayeeNotFound, id)
				}
				return err
			}
			if payee.TransferAccountID != nil {
				return ErrTransferPayee
			}
		}
		var err error
		result, err = s.repo.MergeTx(ctx, tx, budgetId, req.TargetID, sourceIds)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
