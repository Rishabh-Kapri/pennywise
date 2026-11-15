package model

import (
	"time"

	"github.com/google/uuid"
)

// BudgetRole is what a member may do in a budget (budget_members.role).
type BudgetRole string

const (
	// BudgetRoleOwner created the budget. One per budget. The budget runs on
	// the owner's AI keys, and only the owner manages members and AI settings.
	BudgetRoleOwner BudgetRole = "owner"
	// BudgetRoleEditor can do everything in the budget except manage members
	// and AI settings.
	BudgetRoleEditor BudgetRole = "editor"
	// BudgetRoleViewer can read the budget and change nothing.
	BudgetRoleViewer BudgetRole = "viewer"
)

// Valid reports whether r is a known role.
func (r BudgetRole) Valid() bool {
	switch r {
	case BudgetRoleOwner, BudgetRoleEditor, BudgetRoleViewer:
		return true
	}
	return false
}

// Invitable reports whether a person can be given r. Ownership isn't handed
// out: there is one owner, the person who created the budget.
func (r BudgetRole) Invitable() bool {
	return r == BudgetRoleEditor || r == BudgetRoleViewer
}

// CanWrite reports whether r may change the budget's data.
func (r BudgetRole) CanWrite() bool {
	return r == BudgetRoleOwner || r == BudgetRoleEditor
}

// BudgetMember is one person in a budget, with what their login shows about
// them. Name, Email and Picture are empty for a user with no Google identity.
type BudgetMember struct {
	UserID    uuid.UUID  `json:"userId"`
	Role      BudgetRole `json:"role"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Picture   string     `json:"picture,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// BudgetInviteTTL is how long an invite stays open.
const BudgetInviteTTL = 14 * 24 * time.Hour

// BudgetInvite is an open invitation to a budget (budget_invites).
type BudgetInvite struct {
	ID        uuid.UUID  `json:"id"`
	BudgetID  uuid.UUID  `json:"budgetId"`
	Email     string     `json:"email"`
	Role      BudgetRole `json:"role"`
	InvitedBy uuid.UUID  `json:"invitedBy"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	// BudgetName and InviterName are filled for the invitee's list, so they
	// can tell what they're joining.
	BudgetName  string `json:"budgetName,omitempty"`
	InviterName string `json:"inviterName,omitempty"`
}

// CreateBudgetInviteRequest is POST /api/budget-invites.
type CreateBudgetInviteRequest struct {
	Email string     `json:"email"`
	Role  BudgetRole `json:"role"`
}

// UpdateBudgetMemberRequest is PUT /api/budget-members/:userId.
type UpdateBudgetMemberRequest struct {
	Role BudgetRole `json:"role"`
}

// UpdateBudgetRequest is PATCH /api/budgets/:id. Nil fields are left alone.
// Older clients send the whole budget; fields other than these are ignored.
type UpdateBudgetRequest struct {
	Name       *string `json:"name"`
	IsSelected *bool   `json:"isSelected"`
}
