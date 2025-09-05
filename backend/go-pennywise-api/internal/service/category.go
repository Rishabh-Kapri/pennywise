package service

import (
	"context"
	"fmt"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type CategoryService interface {
	GetAll(ctx context.Context) ([]model.Category, error)
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
}

func NewCategoryService(r repository.CategoryRepository, mbR repository.MonthlyBudgetRepository) CategoryService {
	return &categoryService{repo: r, monthlyBudgetRepo: mbR}
}

func (s *categoryService) GetAll(ctx context.Context) ([]model.Category, error) {
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)
	return s.repo.GetAll(ctx, budgetId)
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
	budgetId, _ := ctx.Value("budgetId").(uuid.UUID)

	foundMonthlyBudget, err := s.monthlyBudgetRepo.GetByCatIdAndMonth(ctx, budgetId, categoryId, month)
	if err != nil {
		return err
		// if errors.Is(err, pgx.ErrNoRows) {
		// 	log.Printf("Monthly budget not found for month: %v: %v", month, err.Error())
		// 	// create monthly budget for month
		// 	// find previous month budget
		// 	monthlyBudget := model.MonthlyBudget{
		// 		Month:            month,
		// 		BudgetID:         budgetId,
		// 		CategoryID:       categoryId,
		// 		Budgeted:         0,
		// 		CarryoverBalance: newBudgeted,
		// 	}
		// 	err = s.monthlyBudgetRepo.Create(ctx, budgetId, monthlyBudget)
		// 	if err != nil {
		// 		return err
		// 	}
		// } else {
		// 	return err
		// }
	}
	// if monthly budget is found, update it
	if foundMonthlyBudget != nil {
		err = s.monthlyBudgetRepo.UpdateBudgetedByCatIdAndMonth(ctx, budgetId, categoryId, month, newBudgeted)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("No monthly budget found for category %v and month %v", categoryId, month)
	}
	return nil
}
