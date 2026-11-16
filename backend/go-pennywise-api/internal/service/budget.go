package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
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
	dbTxn        repository.DBTxn
}

func NewBudgetService(
	repo repository.BudgetRepository,
	payeeRepo repository.PayeesRepository,
	catRepo repository.CategoryRepository,
	catGroupRepo repository.CategoryGroupRepository,
	dbTxn repository.DBTxn,
) BudgetService {
	return &budgetService{repo, payeeRepo, catRepo, catGroupRepo, dbTxn}
}

func (s *budgetService) GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error) {
	return s.repo.GetAll(ctx, userID)
}

func (s *budgetService) Create(ctx context.Context, input model.CreateBudgetRequest, userID uuid.UUID) (*model.Budget, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("budget name is required")
	}
	existingBudgets, err := s.repo.GetAll(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error checking existing budgets: %w", err)
	}
	isFirstBudget := len(existingBudgets) == 0

	var created *model.Budget
	err = s.dbTxn.Do(ctx, func(ctx context.Context) error {
		var err error
		created, _, err = newBudget(ctx, budgetRepos{
			budgets:    s.repo,
			catGroups:  s.catGroupRepo,
			categories: s.catRepo,
			payees:     s.payeeRepo,
		}, name, userID, input.TemplateGroups, isFirstBudget)
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Every budget has an inflow category and a starting balance payee with these names.
const (
	inflowCategoryName       = "Inflow: Ready to Assign"
	startingBalancePayeeName = "Starting Balance"
)

// budgetRepos are what creating a budget writes to.
type budgetRepos struct {
	budgets    repository.BudgetRepository
	catGroups  repository.CategoryGroupRepository
	categories repository.CategoryRepository
	payees     repository.PayeesRepository
}

// newBudget creates a budget with what every budget has: the internal master
// and credit card payments groups, the inflow category and the starting
// balance payee, plus the template groups and categories. Run it inside a
// database transaction. It returns the budget and the id of every category it
// made (the inflow one included) by name.
func newBudget(
	ctx context.Context,
	r budgetRepos,
	name string,
	userID uuid.UUID,
	templates []model.BudgetTemplateGroup,
	selected bool,
) (*model.Budget, map[string]uuid.UUID, error) {
	budget, err := r.budgets.Create(ctx, name, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating budget: %w", err)
	}
	masterGroup, err := r.catGroups.Create(ctx, model.CategoryGroup{
		Name: "Internal Master Category", BudgetID: budget.ID, IsSystem: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating internal master category group: %w", err)
	}
	ccGroup, err := r.catGroups.Create(ctx, model.CategoryGroup{
		Name: "Credit Card Payments", BudgetID: budget.ID, IsSystem: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating credit card category group: %w", err)
	}
	startingBalPayee, err := r.payees.Create(ctx, model.Payee{Name: startingBalancePayeeName, BudgetID: budget.ID})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating starting balance payee: %w", err)
	}
	inflow, err := r.categories.Create(ctx, model.Category{
		Name: inflowCategoryName, BudgetID: budget.ID, CategoryGroupID: masterGroup.ID, IsSystem: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating inflow category: %w", err)
	}

	categoryIDs := map[string]uuid.UUID{inflow.Name: inflow.ID}
	for _, templateGroup := range templates {
		groupName := strings.TrimSpace(templateGroup.Name)
		if groupName == "" {
			continue
		}
		group, err := r.catGroups.Create(ctx, model.CategoryGroup{Name: groupName, BudgetID: budget.ID})
		if err != nil {
			return nil, nil, fmt.Errorf("error creating category group %q: %w", groupName, err)
		}
		for _, templateCategory := range templateGroup.Categories {
			categoryName := strings.TrimSpace(templateCategory.Name)
			if categoryName == "" {
				continue
			}
			category, err := r.categories.Create(ctx, model.Category{
				Name: categoryName, BudgetID: budget.ID, CategoryGroupID: group.ID,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("error creating category %q: %w", categoryName, err)
			}
			categoryIDs[categoryName] = category.ID
		}
	}

	updated := model.Budget{
		Name:       budget.Name,
		IsSelected: selected,
		Metadata: model.BudgetMetadata{
			InflowCategoryID:   inflow.ID,
			CCGroupID:          ccGroup.ID,
			StartingBalPayeeID: startingBalPayee.ID,
		},
	}
	if err := r.budgets.UpdateById(ctx, budget.ID, updated); err != nil {
		return nil, nil, fmt.Errorf("error updating budget metadata: %w", err)
	}
	budget.IsSelected = updated.IsSelected
	budget.Metadata = updated.Metadata
	return budget, categoryIDs, nil
}

func (s *budgetService) UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error {
	return s.repo.UpdateById(ctx, id, budget)
}
