package service

import (
	"context"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// maxCatchUpPerRule caps how many past occurrences a single rule can
// materialize in one pass, so a rule left paused for years (or seeded with a
// far-past start date) can't flood the ledger in one tick.
const maxCatchUpPerRule = 60

type RecurringTransactionService interface {
	GetAll(ctx context.Context) ([]model.RecurringTransaction, error)
	Create(ctx context.Context, rule model.RecurringTransaction) (*model.RecurringTransaction, error)
	Update(ctx context.Context, id uuid.UUID, rule model.RecurringTransaction) (*model.RecurringTransaction, error)
	DeleteById(ctx context.Context, id uuid.UUID) error
	// RunDue materializes everything due for the budget in context.
	RunDue(ctx context.Context) (*model.RecurringRunResult, error)
	// RunDueAllBudgets materializes everything due across all budgets; used
	// by the background scheduler, which has no budget in context.
	RunDueAllBudgets(ctx context.Context) (*model.RecurringRunResult, error)
}

type recurringTransactionService struct {
	repo               repository.RecurringTransactionRepository
	transactionService TransactionService
}

func NewRecurringTransactionService(
	repo repository.RecurringTransactionRepository,
	transactionService TransactionService,
) RecurringTransactionService {
	return &recurringTransactionService{repo: repo, transactionService: transactionService}
}

func (s *recurringTransactionService) GetAll(ctx context.Context) ([]model.RecurringTransaction, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *recurringTransactionService) Create(
	ctx context.Context,
	rule model.RecurringTransaction,
) (*model.RecurringTransaction, error) {
	budgetId := utils.MustBudgetID(ctx)
	rule.BudgetID = budgetId
	if rule.IntervalCount == 0 {
		rule.IntervalCount = 1
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, budgetId, rule)
}

func (s *recurringTransactionService) Update(
	ctx context.Context,
	id uuid.UUID,
	rule model.RecurringTransaction,
) (*model.RecurringTransaction, error) {
	budgetId := utils.MustBudgetID(ctx)
	rule.BudgetID = budgetId
	if rule.IntervalCount == 0 {
		rule.IntervalCount = 1
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, budgetId, id, rule); err != nil {
		return nil, err
	}
	return s.repo.GetById(ctx, budgetId, id)
}

func (s *recurringTransactionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.DeleteById(ctx, budgetId, id)
}

func (s *recurringTransactionService) RunDue(ctx context.Context) (*model.RecurringRunResult, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.runDue(ctx, &budgetId)
}

func (s *recurringTransactionService) RunDueAllBudgets(ctx context.Context) (*model.RecurringRunResult, error) {
	return s.runDue(ctx, nil)
}

func (s *recurringTransactionService) runDue(
	ctx context.Context,
	budgetId *uuid.UUID,
) (*model.RecurringRunResult, error) {
	today := time.Now().Format("2006-01-02")
	rules, err := s.repo.GetDue(ctx, today, budgetId)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "failed to load due recurring transactions", err)
	}

	result := &model.RecurringRunResult{}
	for _, rule := range rules {
		created, err := s.materializeRule(ctx, rule, today)
		result.Created += created
		if err != nil {
			// one bad rule must not stop the rest of the run
			result.Skipped++
			logger.Logger(ctx).Error(
				"failed to materialize recurring transaction",
				"ruleId", rule.ID,
				"budgetId", rule.BudgetID,
				"error", err,
			)
		}
	}
	return result, nil
}

// materializeRule creates every occurrence that is due on or before today,
// advancing the rule after each one so a failure part-way through does not
// duplicate the transactions already written.
func (s *recurringTransactionService) materializeRule(
	ctx context.Context,
	rule model.RecurringTransaction,
	today string,
) (int, error) {
	// the scheduler runs without a budget in context, so scope it per rule
	ruleCtx := utils.WithBudgetID(ctx, rule.BudgetID)

	created := 0
	occurrence := rule.NextDate
	for i := 0; i < maxCatchUpPerRule; i++ {
		if string(occurrence) > today {
			break
		}
		if rule.EndDate != nil && *rule.EndDate != "" && string(occurrence) > string(*rule.EndDate) {
			break
		}

		txn := model.Transaction{
			BudgetID:   rule.BudgetID,
			Date:       occurrence,
			AccountID:  &rule.AccountID,
			PayeeID:    rule.PayeeID,
			CategoryID: rule.CategoryID,
			Amount:     rule.Amount,
			Note:       rule.Note,
			Status:     model.TransactionStatusManual,
		}
		if _, err := s.transactionService.Create(ruleCtx, txn); err != nil {
			return created, errs.Wrap(errs.CodeTransactionCreateFailed, "failed to create recurring transaction", err)
		}

		next, err := rule.Frequency.Next(occurrence, rule.IntervalCount)
		if err != nil {
			return created, err
		}
		if err := s.repo.MarkRun(ruleCtx, nil, rule.ID, next, occurrence); err != nil {
			return created, errs.Wrap(errs.CodeInternalError, "failed to advance recurring transaction", err)
		}

		created++
		occurrence = next
	}
	return created, nil
}
