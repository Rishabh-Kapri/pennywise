package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
)

type BudgetService interface {
	GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error)
	Create(ctx context.Context, input model.CreateBudgetRequest, userID uuid.UUID) (*model.Budget, error)
	// Update renames a budget (owner only) and selects it for userID (any
	// member). A name equal to the current one isn't a rename, so older
	// clients that send the whole budget on select still work for members.
	Update(ctx context.Context, userID, id uuid.UUID, req model.UpdateBudgetRequest) error
}

type budgetService struct {
	repo         repository.BudgetRepository
	members      repository.BudgetMemberRepository
	payeeRepo    repository.PayeesRepository
	catRepo      repository.CategoryRepository
	catGroupRepo repository.CategoryGroupRepository
	dbTxn        repository.DBTxn
}

func NewBudgetService(
	repo repository.BudgetRepository,
	members repository.BudgetMemberRepository,
	payeeRepo repository.PayeesRepository,
	catRepo repository.CategoryRepository,
	catGroupRepo repository.CategoryGroupRepository,
	dbTxn repository.DBTxn,
) BudgetService {
	return &budgetService{repo, members, payeeRepo, catRepo, catGroupRepo, dbTxn}
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
	err := s.dbTxn.Do(ctx, func(ctx context.Context) error {
		// 1. create budget
		budget, err := s.repo.Create(ctx, name, userID)
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
		createdGroup, err := s.catGroupRepo.Create(ctx, catGroup)
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
		createdCCGroup, err := s.catGroupRepo.Create(ctx, ccGroup)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error creating internal cc category group: %v", err)
		}
		// 3. create starting balance payee
		startingBalPayee := model.Payee{
			Name:     "Starting Balance",
			BudgetID: budget.ID,
		}
		createdPayee, err := s.payeeRepo.Create(ctx, startingBalPayee)
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
		createdCat, err := s.catRepo.Create(ctx, cat)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error creating internal master category: %v", err)
		}

		for _, templateGroup := range input.TemplateGroups {
			groupName := strings.TrimSpace(templateGroup.Name)
			if groupName == "" {
				continue
			}

			createdTemplateGroup, err := s.catGroupRepo.Create(ctx, model.CategoryGroup{
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

				_, err = s.catRepo.Create(ctx, model.Category{
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
			Name: budget.Name,
			Metadata: model.BudgetMetadata{
				InflowCategoryID:   createdCat.ID,
				CCGroupID:          createdCCGroup.ID,
				StartingBalPayeeID: createdPayee.ID,
			},
		}
		err = s.repo.UpdateById(ctx, budget.ID, updatedBudget)
		if err != nil {
			return fmt.Errorf("budgetService.Create; error updating budget metadata: %v", err)
		}
		createdBudget.Metadata = updatedBudget.Metadata
		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdBudget, nil
}

func (s *budgetService) Update(ctx context.Context, userID, id uuid.UUID, req model.UpdateBudgetRequest) error {
	role, err := s.members.Role(ctx, id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return errs.NewWith(errs.CodeBudgetNotFound, "budget not found", errs.WithStatus(http.StatusNotFound))
	}
	if err != nil {
		return err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		current, err := s.repo.GetById(ctx, id)
		if err != nil {
			return err
		}
		if name != current.Name {
			if role != model.BudgetRoleOwner {
				return errOwnerOnly
			}
			if name == "" {
				return errs.NewWith(errs.CodeInvalidArgument, "budget name is required", errs.WithStatus(http.StatusBadRequest))
			}
			if err := s.repo.Rename(ctx, id, name); err != nil {
				return err
			}
		}
	}

	// Unselecting is implied by selecting another budget, so false is ignored.
	if req.IsSelected != nil && *req.IsSelected {
		return s.members.SetSelected(ctx, userID, id)
	}
	return nil
}
