package service

import (
	"context"
	"fmt"
	"log"

	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/google/uuid"
)

type BudgetService interface {
	GetAll(ctx context.Context) ([]model.Budget, error)
	Create(ctx context.Context, name string) error
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

func (s *budgetService) GetAll(ctx context.Context) ([]model.Budget, error) {
	return s.repo.GetAll(ctx)
}

func (s *budgetService) Create(ctx context.Context, name string) error {
	tx, err := s.repo.GetPgxTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. create budget
	createdBudget, err := s.repo.Create(ctx, tx, name)
	if err != nil {
		return fmt.Errorf("budgetService.Create; error creating budget: %v", err)
	}
	// 2. create master interal category group for credit card and inflow category group
	catGroup := model.CategoryGroup{
		Name:     "Internal Master Category",
		BudgetID: createdBudget.ID,
		Hidden:   false,
		IsSystem: true,
	}
	createdGroup, err := s.catGroupRepo.Create(ctx, tx, catGroup)
	log.Printf("%+v", createdGroup)
	if err != nil {
		return fmt.Errorf("budgetService.Create; error creating internal master category group: %v", err)
	}
	ccGroup := model.CategoryGroup{
		Name:     "Credit Card Payments",
		BudgetID: createdBudget.ID,
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
		BudgetID: createdBudget.ID,
	}
	createdPayee, err := s.payeeRepo.Create(ctx, tx, startingBalPayee)
	if err != nil {
		return fmt.Errorf("budgetService.Create; error creating internal starting balance payee: %v", err)
	}
	// 4. create master internal category
	cat := model.Category{
		Name:            "Inflow: Ready to Assign",
		BudgetID:        createdBudget.ID,
		CategoryGroupID: createdGroup.ID,
		Hidden:          false,
		IsSystem:        true,
	}
	log.Printf("%+v", cat)
	_, err = s.catRepo.Create(ctx, tx, cat)
	if err != nil {
		return fmt.Errorf("budgetService.Create; error creating internal master category: %v", err)
	}
	// 5. update budget with metadata
	updatedBudget := model.Budget{
		Name:       createdBudget.Name,
		IsSelected: createdBudget.IsSelected,
		Metadata: model.BudgetMetadata{
			InflowCategoryID:   cat.ID,
			CCGroupID:          createdCCGroup.ID,
			StartingBalPayeeID: createdPayee.ID,
		},
	}
	err = s.repo.UpdateById(ctx, tx, createdBudget.ID, updatedBudget)
	if err != nil {
		return fmt.Errorf("budgetService.Create; error updating budget metadata: %v", err)
	}

	return tx.Commit(ctx)
}

func (s *budgetService) UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error {
	return s.repo.UpdateById(ctx, nil, id, budget)
}


