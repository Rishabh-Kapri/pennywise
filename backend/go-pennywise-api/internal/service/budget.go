package service

import (
	"context"
	"fmt"
	"strings"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BudgetService interface {
	GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error)
	Create(ctx context.Context, input model.CreateBudgetRequest, userID uuid.UUID) (*model.Budget, error)
	UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error
}

type budgetService struct {
	repo         repository.BudgetRepository
	payeeRepo    repository.PayeesRepository
	catRepo      repository.CategoryRepository
	catGroupRepo repository.CategoryGroupRepository
}

func NewBudgetService(
	repo repository.BudgetRepository,
	payeeRepo repository.PayeesRepository,
	catRepo repository.CategoryRepository,
	catGroupRepo repository.CategoryGroupRepository,
) BudgetService {
	return &budgetService{repo, payeeRepo, catRepo, catGroupRepo}
}

func (s *budgetService) GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error) {
	return s.repo.GetAll(ctx, userID)
}

func (s *budgetService) Create(ctx context.Context, input model.CreateBudgetRequest, userID uuid.UUID) (*model.Budget, error) {
	var createdBudget *model.Budget
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("budget name is required")
	}
	existingBudgets, err := s.repo.GetAll(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("budgetService.Create; error checking existing budgets: %v", err)
	}
	isFirstBudget := len(existingBudgets) == 0

	err = utils.WithTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		// 1. create budget
		budget, err := s.repo.Create(ctx, tx, name, userID)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error creating budget: %v", err)
		}
		createdBudget = budget
		// 2. create master interal category group for credit card and inflow category group
		catGroup := model.CategoryGroup{
			Name:     "Internal Master Category",
			BudgetID: budget.ID,
			Hidden:   false,
			IsSystem: true,
		}
		createdGroup, err := s.catGroupRepo.Create(ctx, tx, catGroup)
		logger.Logger(ctx).Debug("created internal master category group", "group", createdGroup)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error creating internal master category group: %v", err)
		}
		ccGroup := model.CategoryGroup{
			Name:     "Credit Card Payments",
			BudgetID: budget.ID,
			Hidden:   false,
			IsSystem: true,
		}
		createdCCGroup, err := s.catGroupRepo.Create(ctx, tx, ccGroup)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error creating internal cc category group: %v", err)
		}
		// 3. create starting balance payee
		startingBalPayee := model.Payee{
			Name:     "Starting Balance",
			BudgetID: budget.ID,
		}
		createdPayee, err := s.payeeRepo.Create(ctx, tx, startingBalPayee)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error creating internal starting balance payee: %v", err)
		}
		// 4. create master internal category
		cat := model.Category{
			Name:            "Inflow: Ready to Assign",
			BudgetID:        budget.ID,
			CategoryGroupID: createdGroup.ID,
			Hidden:          false,
			IsSystem:        true,
		}
		logger.Logger(ctx).Debug("created inflow category", "category", cat)
		createdCat, err := s.catRepo.Create(ctx, tx, cat)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error creating internal master category: %v", err)
		}

		for _, templateGroup := range input.TemplateGroups {
			groupName := strings.TrimSpace(templateGroup.Name)
			if groupName == "" {
				continue
			}

			createdTemplateGroup, err := s.catGroupRepo.Create(ctx, tx, model.CategoryGroup{
				Name:     groupName,
				BudgetID: budget.ID,
				Hidden:   false,
				IsSystem: false,
			})
			if err != nil {
				return fmt.Errorf("budgetService.Create; error creating template category group: %v", err)
			}

			for _, templateCategory := range templateGroup.Categories {
				categoryName := strings.TrimSpace(templateCategory.Name)
				if categoryName == "" {
					continue
				}

				_, err = s.catRepo.Create(ctx, tx, model.Category{
					Name:            categoryName,
					BudgetID:        budget.ID,
					CategoryGroupID: createdTemplateGroup.ID,
					Hidden:          false,
					IsSystem:        false,
				})
				if err != nil {
					return fmt.Errorf("budgetService.Create; error creating template category: %v", err)
				}
			}
		}

		// 5. update budget with metadata
		updatedBudget := model.Budget{
			Name:       budget.Name,
			IsSelected: isFirstBudget,
			Metadata: model.BudgetMetadata{
				InflowCategoryID:   createdCat.ID,
				CCGroupID:          createdCCGroup.ID,
				StartingBalPayeeID: createdPayee.ID,
			},
		}
		err = s.repo.UpdateById(ctx, tx, budget.ID, updatedBudget)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error updating budget metadata: %v", err)
		}
		createdBudget.IsSelected = updatedBudget.IsSelected
		createdBudget.Metadata = updatedBudget.Metadata
		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdBudget, nil
}

func (s *budgetService) UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error {
	return s.repo.UpdateById(ctx, nil, id, budget)
}
