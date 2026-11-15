package repository

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

// ErrInviteExists: the budget already has an open invite for that email.
var ErrInviteExists = errors.New("this email already has an open invite to the budget")

// BudgetMemberRepository is budget_members: who may open a budget, and as what.
type BudgetMemberRepository interface {
	// Role is userID's role in a budget that isn't deleted, or ErrNotFound.
	Role(ctx context.Context, budgetID, userID uuid.UUID) (model.BudgetRole, error)
	// List returns the budget's members, the owner first, then by join date.
	List(ctx context.Context, budgetID uuid.UUID) ([]model.BudgetMember, error)
	// Add makes userID a member. It reports false, changing nothing, when they
	// already are one.
	Add(ctx context.Context, budgetID, userID uuid.UUID, role model.BudgetRole, invitedBy *uuid.UUID) (bool, error)
	// UpdateRole changes a member's role. The owner's row is never changed:
	// ErrNotFound for the owner or a non-member.
	UpdateRole(ctx context.Context, budgetID, userID uuid.UUID, role model.BudgetRole) error
	// Remove takes userID out of the budget. The owner can't be removed:
	// ErrNotFound for the owner or a non-member.
	Remove(ctx context.Context, budgetID, userID uuid.UUID) error
	// SetSelected marks budgetID as the budget userID has open, and unmarks
	// their others. ErrNotFound when they aren't a member.
	SetSelected(ctx context.Context, userID, budgetID uuid.UUID) error
	// EmailsOfUser returns the lowercase emails of userID's Google logins.
	EmailsOfUser(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// BudgetInviteRepository is budget_invites. "Open" means not accepted,
// declined or revoked, and not expired.
type BudgetInviteRepository interface {
	// Create saves an invite; ErrInviteExists if one is open for the email.
	Create(ctx context.Context, invite model.BudgetInvite) (*model.BudgetInvite, error)
	// ListByBudget returns a budget's open invites, newest first.
	ListByBudget(ctx context.Context, budgetID uuid.UUID) ([]model.BudgetInvite, error)
	// ListForEmails returns the open invites to any of emails, with the
	// budget's and inviter's names, newest first. Deleted budgets are left out.
	ListForEmails(ctx context.Context, emails []string) ([]model.BudgetInvite, error)
	// GetOpen returns an open invite, or ErrNotFound.
	GetOpen(ctx context.Context, id uuid.UUID) (*model.BudgetInvite, error)
	// MarkAccepted closes an open invite as accepted by userID. ErrNotFound
	// when it's no longer open.
	MarkAccepted(ctx context.Context, id, userID uuid.UUID) error
	// MarkDeclined closes an open invite as declined. ErrNotFound when it's no
	// longer open.
	MarkDeclined(ctx context.Context, id uuid.UUID) error
	// Revoke closes one of the budget's open invites. ErrNotFound when it's
	// not open or belongs to another budget.
	Revoke(ctx context.Context, budgetID, id uuid.UUID) error
	// RevokeForEmails closes the budget's open invites to any of emails, used
	// when a member is removed so an old invite can't bring them back.
	RevokeForEmails(ctx context.Context, budgetID uuid.UUID, emails []string) error
}
