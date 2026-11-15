package model

import (
	"time"

	"github.com/google/uuid"
)

type BudgetMetadata struct {
	InflowCategoryID   uuid.UUID `json:"inflowCategoryId" validate:"required"`
	StartingBalPayeeID uuid.UUID `json:"startingBalPayeeId" validate:"required"`
	CCGroupID          uuid.UUID `json:"ccGroupId" validate:"required"`
}

type Budget struct {
	ID uuid.UUID `json:"id"`
	// UserID is the budget's owner.
	UserID uuid.UUID `json:"userId"`
	Name   string    `json:"name"`
	// IsSelected, Role and MemberCount are about the user reading the list:
	// whether it's the budget they last opened, and what they may do in it.
	IsSelected  bool             `json:"isSelected"`
	Role        BudgetRole       `json:"role,omitempty"`
	MemberCount int              `json:"memberCount,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	Metadata    BudgetMetadata   `json:"metadata"`
	Onboarding  BudgetOnboarding `json:"onboarding"`
}

// Onboarding wizard steps, in order. OnboardingStepDone means the wizard has
// finished; an empty step means it hasn't started.
const (
	OnboardingStepWelcome  = "welcome"
	OnboardingStepAccounts = "accounts"
	OnboardingStepBills    = "bills"
	OnboardingStepAI       = "ai"
	OnboardingStepEmail    = "email"
	OnboardingStepAssign   = "assign"
	OnboardingStepDone     = "done"
)

var OnboardingSteps = []string{
	OnboardingStepWelcome,
	OnboardingStepAccounts,
	OnboardingStepBills,
	OnboardingStepAI,
	OnboardingStepEmail,
	OnboardingStepAssign,
	OnboardingStepDone,
}

// BudgetOnboarding is budgets.onboarding.
type BudgetOnboarding struct {
	Step               string     `json:"step"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
	SkippedSteps       []string   `json:"skippedSteps,omitempty"`
	ChecklistDismissed bool       `json:"checklistDismissed"`
	// EmailsImportedAt is when this budget's onboarding import started. Step 4
	// offers the import only once.
	EmailsImportedAt *time.Time `json:"emailsImportedAt,omitempty"`
}

// UpdateBudgetOnboardingRequest is PATCH /api/onboarding. Nil fields are left
// unchanged.
type UpdateBudgetOnboardingRequest struct {
	Step               *string  `json:"step"`
	SkippedSteps       []string `json:"skippedSteps"`
	ChecklistDismissed *bool    `json:"checklistDismissed"`
	EmailsImported     *bool    `json:"emailsImported"`
}

// OnboardingEmailBackfillRequest is POST /api/onboarding/email-backfill.
type OnboardingEmailBackfillRequest struct {
	Days int `json:"days"`
}

// OnboardingEmailBackfillResponse reports what was queued.
type OnboardingEmailBackfillResponse struct {
	EmailCount int    `json:"emailCount"`
	WorkflowID string `json:"workflowId,omitempty"`
}

type BudgetTemplateCategory struct {
	Name string `json:"name"`
}

type BudgetTemplateGroup struct {
	Name       string                   `json:"name"`
	Categories []BudgetTemplateCategory `json:"categories"`
}

type CreateBudgetRequest struct {
	Name           string                `json:"name"`
	TemplateGroups []BudgetTemplateGroup `json:"templateGroups"`
}
