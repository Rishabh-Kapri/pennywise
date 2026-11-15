package service

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// BudgetSharingService manages who is in a budget. The methods without a
// budget argument act on the request's budget (BudgetIdMiddleware); the
// invitee's side (MyInvites, AcceptInvite, DeclineInvite, Leave) works from
// the user, because the invitee isn't a member yet.
type BudgetSharingService interface {
	Members(ctx context.Context) ([]model.BudgetMember, error)
	// ChangeRole and RemoveMember are for the owner. The owner's own row can't
	// be changed or removed.
	ChangeRole(ctx context.Context, userID uuid.UUID, role model.BudgetRole) error
	RemoveMember(ctx context.Context, userID uuid.UUID) error

	// Invites lists the budget's open invites; Invite and RevokeInvite are for
	// the owner.
	Invites(ctx context.Context) ([]model.BudgetInvite, error)
	Invite(ctx context.Context, req model.CreateBudgetInviteRequest) (*model.BudgetInvite, error)
	RevokeInvite(ctx context.Context, inviteID uuid.UUID) error

	// MyInvites lists the open invites to any of userID's login emails.
	MyInvites(ctx context.Context, userID uuid.UUID) ([]model.BudgetInvite, error)
	// AcceptInvite makes userID a member with the invite's role and returns
	// the budget's ID.
	AcceptInvite(ctx context.Context, userID, inviteID uuid.UUID) (uuid.UUID, error)
	DeclineInvite(ctx context.Context, userID, inviteID uuid.UUID) error
	// Leave takes userID out of a budget they don't own.
	Leave(ctx context.Context, userID, budgetID uuid.UUID) error
}

type budgetSharingService struct {
	members    repository.BudgetMemberRepository
	invites    repository.BudgetInviteRepository
	dbTxn      repository.DBTxn
	isDemoUser IsDemoUser
	now        func() time.Time
}

func NewBudgetSharingService(
	members repository.BudgetMemberRepository,
	invites repository.BudgetInviteRepository,
	dbTxn repository.DBTxn,
	isDemoUser IsDemoUser,
) BudgetSharingService {
	return &budgetSharingService{
		members:    members,
		invites:    invites,
		dbTxn:      dbTxn,
		isDemoUser: isDemoUser,
		now:        time.Now,
	}
}

var (
	errOwnerOnly = errs.NewWith(
		errs.CodeBudgetForbidden, "only the budget's owner can do this", errs.WithStatus(http.StatusForbidden),
	)
	errMemberNotFound = errs.NewWith(
		errs.CodeBudgetMemberNotFound, "no such member (the owner can't be changed or removed)",
		errs.WithStatus(http.StatusNotFound),
	)
	errInviteNotFound = errs.NewWith(
		errs.CodeBudgetInviteNotFound, "this invite is no longer open", errs.WithStatus(http.StatusNotFound),
	)
	errInvalidRole = errs.NewWith(
		errs.CodeInvalidArgument, "role must be editor or viewer", errs.WithStatus(http.StatusBadRequest),
	)
	errDemoSharing = errs.NewWith(
		errs.CodeBudgetForbidden, "the demo account can't share budgets", errs.WithStatus(http.StatusForbidden),
	)
)

// requireOwner backs up the routes' RequireBudgetRole, so a service call from
// a new route can't skip the check.
func requireOwner(ctx context.Context) error {
	if utils.VerifiedInternalFromContext(ctx) || utils.BudgetRoleFromContext(ctx) == model.BudgetRoleOwner {
		return nil
	}
	return errOwnerOnly
}

func (s *budgetSharingService) Members(ctx context.Context) ([]model.BudgetMember, error) {
	return s.members.List(ctx, utils.MustBudgetID(ctx))
}

func (s *budgetSharingService) ChangeRole(ctx context.Context, userID uuid.UUID, role model.BudgetRole) error {
	if err := requireOwner(ctx); err != nil {
		return err
	}
	if !role.Invitable() {
		return errInvalidRole
	}
	err := s.members.UpdateRole(ctx, utils.MustBudgetID(ctx), userID, role)
	if errors.Is(err, repository.ErrNotFound) {
		return errMemberNotFound
	}
	return err
}

func (s *budgetSharingService) RemoveMember(ctx context.Context, userID uuid.UUID) error {
	if err := requireOwner(ctx); err != nil {
		return err
	}
	return s.remove(ctx, utils.MustBudgetID(ctx), userID)
}

// remove takes a non-owner out of a budget and closes any open invite to
// their emails, so an old invite can't bring them back.
func (s *budgetSharingService) remove(ctx context.Context, budgetID, userID uuid.UUID) error {
	emails, err := s.members.EmailsOfUser(ctx, userID)
	if err != nil {
		return err
	}
	return s.dbTxn.Do(ctx, func(ctx context.Context) error {
		err := s.members.Remove(ctx, budgetID, userID)
		if errors.Is(err, repository.ErrNotFound) {
			return errMemberNotFound
		}
		if err != nil {
			return err
		}
		return s.invites.RevokeForEmails(ctx, budgetID, emails)
	})
}

func (s *budgetSharingService) Invites(ctx context.Context) ([]model.BudgetInvite, error) {
	if err := requireOwner(ctx); err != nil {
		return nil, err
	}
	return s.invites.ListByBudget(ctx, utils.MustBudgetID(ctx))
}

func (s *budgetSharingService) Invite(
	ctx context.Context,
	req model.CreateBudgetInviteRequest,
) (*model.BudgetInvite, error) {
	if err := requireOwner(ctx); err != nil {
		return nil, err
	}
	userID := utils.MustUserID(ctx)
	if s.isDemoUser(ctx, userID) {
		return nil, errDemoSharing
	}
	email, err := normalizeInviteEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if !req.Role.Invitable() {
		return nil, errInvalidRole
	}
	budgetID := utils.MustBudgetID(ctx)

	member, err := s.memberWithEmail(ctx, budgetID, email)
	if err != nil {
		return nil, err
	}
	if member {
		return nil, errs.NewWith(
			errs.CodeBudgetAlreadyMember, "this person is already in the budget",
			errs.WithStatus(http.StatusConflict),
		)
	}

	invite, err := s.invites.Create(ctx, model.BudgetInvite{
		BudgetID:  budgetID,
		Email:     email,
		Role:      req.Role,
		InvitedBy: userID,
		ExpiresAt: s.now().Add(model.BudgetInviteTTL),
	})
	if errors.Is(err, repository.ErrInviteExists) {
		return nil, errs.NewWith(
			errs.CodeBudgetInviteExists, "this email already has an open invite",
			errs.WithStatus(http.StatusConflict),
		)
	}
	return invite, err
}

// memberWithEmail reports whether any member of the budget logs in with email.
func (s *budgetSharingService) memberWithEmail(ctx context.Context, budgetID uuid.UUID, email string) (bool, error) {
	members, err := s.members.List(ctx, budgetID)
	if err != nil {
		return false, err
	}
	for _, m := range members {
		emails, err := s.members.EmailsOfUser(ctx, m.UserID)
		if err != nil {
			return false, err
		}
		if slices.Contains(emails, email) {
			return true, nil
		}
	}
	return false, nil
}

func normalizeInviteEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if email == "" || err != nil || addr.Address != email {
		return "", errs.NewWith(
			errs.CodeInvalidArgument, "enter a valid email address", errs.WithStatus(http.StatusBadRequest),
		)
	}
	return email, nil
}

func (s *budgetSharingService) RevokeInvite(ctx context.Context, inviteID uuid.UUID) error {
	if err := requireOwner(ctx); err != nil {
		return err
	}
	err := s.invites.Revoke(ctx, utils.MustBudgetID(ctx), inviteID)
	if errors.Is(err, repository.ErrNotFound) {
		return errInviteNotFound
	}
	return err
}

func (s *budgetSharingService) MyInvites(ctx context.Context, userID uuid.UUID) ([]model.BudgetInvite, error) {
	emails, err := s.members.EmailsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.invites.ListForEmails(ctx, emails)
}

// openInviteFor returns an open invite addressed to one of userID's emails.
// An invite to someone else reads as not found, so IDs reveal nothing.
func (s *budgetSharingService) openInviteFor(
	ctx context.Context,
	userID, inviteID uuid.UUID,
) (*model.BudgetInvite, error) {
	invite, err := s.invites.GetOpen(ctx, inviteID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errInviteNotFound
	}
	if err != nil {
		return nil, err
	}
	emails, err := s.members.EmailsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(emails, invite.Email) {
		return nil, errInviteNotFound
	}
	return invite, nil
}

func (s *budgetSharingService) AcceptInvite(ctx context.Context, userID, inviteID uuid.UUID) (uuid.UUID, error) {
	if s.isDemoUser(ctx, userID) {
		return uuid.Nil, errDemoSharing
	}
	invite, err := s.openInviteFor(ctx, userID, inviteID)
	if err != nil {
		return uuid.Nil, err
	}
	err = s.dbTxn.Do(ctx, func(ctx context.Context) error {
		// Closing the invite first makes a double accept fail here instead of
		// adding the member twice. Someone already in the budget keeps their
		// role: an invite never demotes or promotes.
		if err := s.invites.MarkAccepted(ctx, invite.ID, userID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return errInviteNotFound
			}
			return err
		}
		inviter := invite.InvitedBy
		_, err := s.members.Add(ctx, invite.BudgetID, userID, invite.Role, &inviter)
		return err
	})
	if err != nil {
		return uuid.Nil, err
	}
	return invite.BudgetID, nil
}

func (s *budgetSharingService) DeclineInvite(ctx context.Context, userID, inviteID uuid.UUID) error {
	invite, err := s.openInviteFor(ctx, userID, inviteID)
	if err != nil {
		return err
	}
	err = s.invites.MarkDeclined(ctx, invite.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return errInviteNotFound
	}
	return err
}

func (s *budgetSharingService) Leave(ctx context.Context, userID, budgetID uuid.UUID) error {
	role, err := s.members.Role(ctx, budgetID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return errs.NewWith(errs.CodeBudgetNotFound, "budget not found", errs.WithStatus(http.StatusNotFound))
	}
	if err != nil {
		return err
	}
	if role == model.BudgetRoleOwner {
		return errs.NewWith(
			errs.CodeBudgetOwnerCannotLeave, "the owner can't leave their own budget",
			errs.WithStatus(http.StatusConflict),
		)
	}
	return s.remove(ctx, budgetID, userID)
}
