package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

var (
	ErrInvalidOnboardingStep = errors.New("invalid onboarding step")
	ErrNoGmailConnected      = errors.New("no Gmail account is connected")
	ErrNoAIProvider          = errors.New("add an AI provider before importing emails")
)

const (
	defaultBackfillDays = 14
	maxBackfillDays     = 30
	// Bounds what one backfill can cost the user's AI provider.
	backfillMaxEmails = 25
)

type OnboardingService interface {
	Get(ctx context.Context) (*model.BudgetOnboarding, error)
	Update(ctx context.Context, req model.UpdateBudgetOnboardingRequest) (*model.BudgetOnboarding, error)
	BackfillEmails(ctx context.Context, userID uuid.UUID, days int) (*model.OnboardingEmailBackfillResponse, error)
}

type onboardingService struct {
	budgetRepo     repository.BudgetRepository
	authRepo       repository.AuthRepository
	googleProvider repository.GoogleProviderRepository
	llmProviders   repository.UserLLMProviderRepository
	gmail          *transport.Client
}

func NewOnboardingService(
	budgetRepo repository.BudgetRepository,
	authRepo repository.AuthRepository,
	googleProvider repository.GoogleProviderRepository,
	llmProviders repository.UserLLMProviderRepository,
	gmail *transport.Client,
) OnboardingService {
	return &onboardingService{budgetRepo, authRepo, googleProvider, llmProviders, gmail}
}

func (s *onboardingService) Get(ctx context.Context) (*model.BudgetOnboarding, error) {
	budget, err := s.budgetRepo.GetById(ctx, nil, utils.MustBudgetID(ctx))
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
	budget, err := s.budgetRepo.GetById(ctx, nil, budgetID)
	if err != nil {
		return nil, err
	}
	onboarding, err := applyOnboardingUpdate(budget.Onboarding, req, time.Now())
	if err != nil {
		return nil, err
	}
	if err := s.budgetRepo.UpdateOnboarding(ctx, nil, budgetID, onboarding); err != nil {
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
	return current, nil
}

type gmailBackfillRequest struct {
	RefreshToken    string                      `json:"refreshToken"`
	OAuthClientType model.GoogleOAuthClientType `json:"oauthClientType"`
	Email           string                      `json:"email"`
	Days            int                         `json:"days"`
	MaxResults      int64                       `json:"maxResults"`
}

// BackfillEmails runs the email pipeline over the user's recent bank alerts,
// which arrived before Pennywise started watching their inbox. Transactions
// show up over the websocket as each email finishes.
func (s *onboardingService) BackfillEmails(
	ctx context.Context,
	userID uuid.UUID,
	days int,
) (*model.OnboardingEmailBackfillResponse, error) {
	if days <= 0 {
		days = defaultBackfillDays
	}
	days = min(days, maxBackfillDays)

	providers, err := s.llmProviders.ListEnabledForBudget(ctx, utils.MustBudgetID(ctx))
	if err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, ErrNoAIProvider
	}

	google, err := s.googleCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}

	return transport.Post[*model.OnboardingEmailBackfillResponse](ctx, s.gmail, "/api/backfill", nil, gmailBackfillRequest{
		RefreshToken:    google.RefreshToken,
		OAuthClientType: google.OAuthClientType,
		Email:           google.Email,
		Days:            days,
		MaxResults:      backfillMaxEmails,
	})
}

// googleCredentials returns the user's first Google login that has Gmail access.
func (s *onboardingService) googleCredentials(ctx context.Context, userID uuid.UUID) (*model.GoogleProviderUser, error) {
	user, err := s.authRepo.GetUserWithProviders(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, provider := range user.Providers {
		if provider.ProviderType != model.GoogleAuthProviderType {
			continue
		}
		creds, err := s.googleProvider.GetUserByGoogleIDAndClientType(
			ctx, provider.ProviderID, model.NormalizeGoogleOAuthClientType(provider.OAuthClientType))
		if err != nil {
			return nil, err
		}
		if creds.GoogleProvider != nil && creds.GoogleProvider.RefreshToken != "" {
			return creds.GoogleProvider, nil
		}
	}
	return nil, ErrNoGmailConnected
}
