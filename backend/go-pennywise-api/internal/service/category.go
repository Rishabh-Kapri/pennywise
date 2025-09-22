package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CategoryService interface {
	GetAll(ctx context.Context) ([]model.Category, error)
	GetInflowBalance(ctx context.Context) (float64, error)
	Search(ctx context.Context, query string) ([]model.Category, error)
	GetById(ctx context.Context, id uuid.UUID) (*model.Category, error)
	Create(ctx context.Context, category model.Category) error
	DeleteById(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, category model.Category) error
	UpdateMonthlyBudget(ctx context.Context, categoryId uuid.UUID, newBudgeted float64, month string) error
}

type categoryService struct {
	repo              repository.CategoryRepository
	monthlyBudgetRepo repository.MonthlyBudgetRepository
	transactionRepo   repository.TransactionRepository
}

func NewCategoryService(r repository.CategoryRepository, mbR repository.MonthlyBudgetRepository, txnR repository.TransactionRepository) CategoryService {
	return &categoryService{repo: r, monthlyBudgetRepo: mbR, transactionRepo: txnR}
}

func (s *categoryService) GetAll(ctx context.Context) ([]model.Category, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *categoryService) GetInflowBalance(ctx context.Context) (float64, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
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
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Search(ctx, budgetId, query)
}

func (s *categoryService) GetById(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetById(ctx, budgetId, id)
}

func (s *categoryService) Create(ctx context.Context, category model.Category) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	category.BudgetID = budgetId
	return s.repo.Create(ctx, category)
}

func (s *categoryService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.DeleteById(ctx, budgetId, id)
}

func (s *categoryService) Update(ctx context.Context, id uuid.UUID, category model.Category) error {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.Update(ctx, budgetId, id, category)
}

// updates the monthly budget for a category for a particular month
// create a new record if it doesn't exist
// gets the carryover from the previous month
func (s *categoryService) UpdateMonthlyBudget(ctx context.Context, categoryId uuid.UUID, newBudgeted float64, month string) error {
	tx, err := s.monthlyBudgetRepo.GetPgxTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)

	exists, err := s.monthlyBudgetRepo.GetByCatIdAndMonth(ctx, tx, budgetId, categoryId, month)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Budget for month %v and category %v doesn't exists, creating new one", month, categoryId)
			// no budget exists for this category
			monthyBudget := model.MonthlyBudget{
				BudgetID:         budgetId,
				CategoryID:       categoryId,
				Month:            month,
				Budgeted:         newBudgeted,
				CarryoverBalance: 0.0,
			}
			err = s.monthlyBudgetRepo.Create(ctx, tx, budgetId, monthyBudget)
			if err != nil {
				return fmt.Errorf("error while creating monthly budget: %w", err)
			}
		} else {
			return fmt.Errorf("error while fetching existing budget: %w", err)
		}
	} else {
		if exists.Budgeted == newBudgeted {
			log.Printf("New and old budgeted same, skipping.")
			return nil
		}
		err = s.monthlyBudgetRepo.UpdateBudgetedByCatIdAndMonth(ctx, tx, budgetId, categoryId, month, newBudgeted)
		if err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
