package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

var (
	ErrInvalidOnboardingStep = errors.New("invalid onboarding step")
	ErrNoGmailConnected      = errors.New("no Gmail account is connected")
	ErrNoAIProvider          = errors.New("add an AI provider before importing emails")
)

type OnboardingService interface {
	Get(ctx context.Context) (*model.BudgetOnboarding, error)
	Update(ctx context.Context, req model.UpdateBudgetOnboardingRequest) (*model.BudgetOnboarding, error)
	BackfillEmails(ctx context.Context, userID uuid.UUID, days int) (*model.OnboardingEmailBackfillResponse, error)
}

type onboardingService struct {
	budgetRepo repository.BudgetRepository
	llmModels  repository.UserLLMModelRepository
}

func NewOnboardingService(
	budgetRepo repository.BudgetRepository,
	llmModels repository.UserLLMModelRepository,
) OnboardingService {
	return &onboardingService{budgetRepo, llmModels}
}

func (s *onboardingService) Get(ctx context.Context) (*model.BudgetOnboarding, error) {
	budget, err := s.budgetRepo.GetById(ctx, utils.MustBudgetID(ctx))
	if err != nil {
		return nil, err
	}
	return &budget.Onboarding, nil
}

func (s *onboardingService) Update(
	ctx context.Context,
	req model.UpdateBudgetOnboardingRequest,
) (*model.BudgetOnboarding, error) {
	budgetID := utils.MustBudgetID(ctx)
	budget, err := s.budgetRepo.GetById(ctx, budgetID)
	if err != nil {
		return nil, err
	}
	onboarding, err := applyOnboardingUpdate(budget.Onboarding, req, time.Now())
	if err != nil {
		return nil, err
	}
	if err := s.budgetRepo.UpdateOnboarding(ctx, budgetID, onboarding); err != nil {
		return nil, err
	}
	return &onboarding, nil
}

// applyOnboardingUpdate merges req into current. Skipped steps accumulate;
// moving to "done" stamps CompletedAt once.
func applyOnboardingUpdate(
	current model.BudgetOnboarding,
	req model.UpdateBudgetOnboardingRequest,
	now time.Time,
) (model.BudgetOnboarding, error) {
	if req.Step != nil {
		if !slices.Contains(model.OnboardingSteps, *req.Step) {
			return current, fmt.Errorf("%w: %q", ErrInvalidOnboardingStep, *req.Step)
		}
		current.Step = *req.Step
		if current.Step == model.OnboardingStepDone && current.CompletedAt == nil {
			current.CompletedAt = &now
		}
	}
	for _, step := range req.SkippedSteps {
		if !slices.Contains(model.OnboardingSteps, step) || step == model.OnboardingStepDone {
			return current, fmt.Errorf("%w: %q", ErrInvalidOnboardingStep, step)
		}
		if !slices.Contains(current.SkippedSteps, step) {
			current.SkippedSteps = append(current.SkippedSteps, step)
		}
	}
	if req.ChecklistDismissed != nil {
		current.ChecklistDismissed = *req.ChecklistDismissed
	}
	if req.EmailsImported != nil && *req.EmailsImported && current.EmailsImportedAt == nil {
		current.EmailsImportedAt = &now
	}
	return current, nil
}

// BackfillEmails should run the email pipeline over the user's recent bank
// alerts. Gmail credentials moved to mail_connections, which only ingest can
// open, so until ingest offers a backfill this always returns
// ErrNoGmailConnected once an AI provider exists.
func (s *onboardingService) BackfillEmails(
	ctx context.Context,
	userID uuid.UUID,
	days int,
) (*model.OnboardingEmailBackfillResponse, error) {
	models, err := s.llmModels.ListUsableForBudget(ctx, utils.MustBudgetID(ctx), model.LLMModelChat)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, ErrNoAIProvider
	}
	return nil, ErrNoGmailConnected
}
