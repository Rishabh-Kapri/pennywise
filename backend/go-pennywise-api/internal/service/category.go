package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

type CategoryService interface {
	GetAll(ctx context.Context) ([]model.Category, error)
	GetInflowBalance(ctx context.Context) (float64, error)
	Search(ctx context.Context, query string) ([]model.Category, error)
	GetById(ctx context.Context, id uuid.UUID) (*model.Category, error)
	Create(ctx context.Context, category model.Category) (*model.Category, error)
	DeleteById(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, category model.Category) error
	UpdateMonthlyBudget(ctx context.Context, categoryId uuid.UUID, newBudgeted float64, month string) error
	UpdateDetails(ctx context.Context, id uuid.UUID, details model.CategoryDetailsUpdate) error
	SuggestDescription(ctx context.Context, id uuid.UUID) (string, error)
}

// ErrCategoryNotFound is returned when a category does not exist in the budget.
var ErrCategoryNotFound = errors.New("category not found")

// ErrDescriptionUnavailable is returned when no AI provider could produce a
// description suggestion.
var ErrDescriptionUnavailable = errors.New("could not suggest a description right now")

// descriptionPayeeLimit caps how many payees ground a description suggestion:
// enough to show what the category is used for without bloating the prompt.
const descriptionPayeeLimit = 12

type categoryService struct {
	repo              repository.CategoryRepository
	monthlyBudgetRepo repository.MonthlyBudgetRepository
	transactionRepo   repository.TransactionRepository
	cipherClient      CipherClient
	dbTxn             repository.DBTxn
}

func NewCategoryService(
	r repository.CategoryRepository,
	mbR repository.MonthlyBudgetRepository,
	txnR repository.TransactionRepository,
	cipher CipherClient,
	dbTxn repository.DBTxn,
) CategoryService {
	return &categoryService{dbTxn: dbTxn, repo: r, monthlyBudgetRepo: mbR, transactionRepo: txnR, cipherClient: cipher}
}

func (s *categoryService) UpdateDetails(ctx context.Context, id uuid.UUID, details model.CategoryDetailsUpdate) error {
	budgetId := utils.MustBudgetID(ctx)
	err := s.repo.UpdateDetails(ctx, budgetId, id, details.Note, details.Description)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrCategoryNotFound
	}
	return err
}

// SuggestDescription asks cipher's LLM chain for a one-sentence description of
// what belongs in the category. It only suggests; nothing is saved until the
// user accepts it through UpdateDetails.
func (s *categoryService) SuggestDescription(ctx context.Context, id uuid.UUID) (string, error) {
	budgetId := utils.MustBudgetID(ctx)
	descCtx, err := s.repo.GetDescriptionContext(ctx, budgetId, id, descriptionPayeeLimit)
	if errors.Is(err, repository.ErrNotFound) {
		return "", ErrCategoryNotFound
	}
	if err != nil {
		return "", err
	}
	if s.cipherClient == nil {
		return "", ErrDescriptionUnavailable
	}

	res, err := s.cipherClient.SuggestCategoryDescription(ctx, *descCtx)
	if err != nil {
		logger.Logger(ctx).Error("category description suggestion failed", "categoryId", id, "error", err)
		return "", ErrDescriptionUnavailable
	}
	if res == nil || res.Description == "" {
		return "", ErrDescriptionUnavailable
	}
	return res.Description, nil
}

func (s *categoryService) GetAll(ctx context.Context) ([]model.Category, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *categoryService) GetInflowBalance(ctx context.Context) (float64, error) {
	budgetId := utils.MustBudgetID(ctx)
	// isSystem := true
	// filter := model.CategoryFilter{
	// 	IsSystem: &isSystem,
	// }
	// categories, err := s.repo.GetByFilter(ctx, budgetId, filter)
	// if err != nil {
	// 	return 0.0, err
	// }
	//
	// balance := 0.0
	// for _, cat := range categories {
	// 	txnFilter := model.TransactionFilter{
	// 		CategoryID: &cat.ID,
	// 	}
	// 	txns, err := s.transactionRepo.GetAll(ctx, budgetId, &txnFilter)
	// 	if err != nil {
	// 		return 0.0, err
	// 	}
	// 	for _, txn := range txns {
	// 		balance += txn.Amount
	// 	}
	// }
	// return balance, nil
	return s.repo.GetInflowBalance(ctx, budgetId)
}

func (s *categoryService) Search(ctx context.Context, query string) ([]model.Category, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Search(ctx, budgetId, query)
}

func (s *categoryService) GetById(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetById(ctx, budgetId, id)
}

func (s *categoryService) Create(ctx context.Context, category model.Category) (*model.Category, error) {
	budgetId := utils.MustBudgetID(ctx)
	category.BudgetID = budgetId
	return s.repo.Create(ctx, category)
}

func (s *categoryService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.DeleteById(ctx, budgetId, id)
}

func (s *categoryService) Update(ctx context.Context, id uuid.UUID, category model.Category) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.Update(ctx, budgetId, id, category)
}

// updates the monthly budget for a category for a particular month
// create a new record if it doesn't exist
// gets the carryover from the previous month
func (s *categoryService) UpdateMonthlyBudget(
	ctx context.Context,
	categoryId uuid.UUID,
	newBudgeted float64,
	month string,
) error {
	budgetId := utils.MustBudgetID(ctx)

	return s.dbTxn.Do(ctx, func(ctx context.Context) error {
		exists, err := s.monthlyBudgetRepo.GetByCatIdAndMonth(ctx, budgetId, categoryId, month)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				logger.Logger(ctx).Info("monthly budget not found, creating", "month", month, "categoryId", categoryId)
				// no budget exists for this category
				monthyBudget := model.MonthlyBudget{
					BudgetID:         budgetId,
					CategoryID:       categoryId,
					Month:            month,
					Budgeted:         newBudgeted,
					CarryoverBalance: 0.0,
				}
				err = s.monthlyBudgetRepo.Create(ctx, budgetId, monthyBudget)
				if err != nil {
					return fmt.Errorf("error while creating monthly budget: %w", err)
				}
			} else {
				return fmt.Errorf("error while fetching existing budget: %w", err)
			}
		} else {
			if exists.Budgeted == newBudgeted {
				logger.Logger(ctx).Debug("budgeted unchanged, skipping")
				return nil
			}
			err = s.monthlyBudgetRepo.UpdateBudgetedByCatIdAndMonth(ctx, budgetId, categoryId, month, newBudgeted)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
