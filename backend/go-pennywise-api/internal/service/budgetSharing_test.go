package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type svcBudgetMemberRepo struct{ mock.Mock }

func (m *svcBudgetMemberRepo) Role(ctx context.Context, budgetID, userID uuid.UUID) (model.BudgetRole, error) {
	args := m.Called(ctx, budgetID, userID)
	return args.Get(0).(model.BudgetRole), args.Error(1)
}
func (m *svcBudgetMemberRepo) List(ctx context.Context, budgetID uuid.UUID) ([]model.BudgetMember, error) {
	args := m.Called(ctx, budgetID)
	return args.Get(0).([]model.BudgetMember), args.Error(1)
}
func (m *svcBudgetMemberRepo) Add(
	ctx context.Context, budgetID, userID uuid.UUID, role model.BudgetRole, invitedBy *uuid.UUID,
) (bool, error) {
	args := m.Called(ctx, budgetID, userID, role, invitedBy)
	return args.Bool(0), args.Error(1)
}
func (m *svcBudgetMemberRepo) UpdateRole(ctx context.Context, budgetID, userID uuid.UUID, role model.BudgetRole) error {
	return m.Called(ctx, budgetID, userID, role).Error(0)
}
func (m *svcBudgetMemberRepo) Remove(ctx context.Context, budgetID, userID uuid.UUID) error {
	return m.Called(ctx, budgetID, userID).Error(0)
}
func (m *svcBudgetMemberRepo) SetSelected(ctx context.Context, userID, budgetID uuid.UUID) error {
	return m.Called(ctx, userID, budgetID).Error(0)
}
func (m *svcBudgetMemberRepo) EmailsOfUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]string), args.Error(1)
}

type svcBudgetInviteRepo struct{ mock.Mock }

func (m *svcBudgetInviteRepo) Create(ctx context.Context, invite model.BudgetInvite) (*model.BudgetInvite, error) {
	args := m.Called(ctx, invite)
	if v := args.Get(0); v != nil {
		return v.(*model.BudgetInvite), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcBudgetInviteRepo) ListByBudget(ctx context.Context, budgetID uuid.UUID) ([]model.BudgetInvite, error) {
	args := m.Called(ctx, budgetID)
	return args.Get(0).([]model.BudgetInvite), args.Error(1)
}
func (m *svcBudgetInviteRepo) ListForEmails(ctx context.Context, emails []string) ([]model.BudgetInvite, error) {
	args := m.Called(ctx, emails)
	return args.Get(0).([]model.BudgetInvite), args.Error(1)
}
func (m *svcBudgetInviteRepo) GetOpen(ctx context.Context, id uuid.UUID) (*model.BudgetInvite, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*model.BudgetInvite), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcBudgetInviteRepo) MarkAccepted(ctx context.Context, id, userID uuid.UUID) error {
	return m.Called(ctx, id, userID).Error(0)
}
func (m *svcBudgetInviteRepo) MarkDeclined(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *svcBudgetInviteRepo) Revoke(ctx context.Context, budgetID, id uuid.UUID) error {
	return m.Called(ctx, budgetID, id).Error(0)
}
func (m *svcBudgetInviteRepo) RevokeForEmails(ctx context.Context, budgetID uuid.UUID, emails []string) error {
	return m.Called(ctx, budgetID, emails).Error(0)
}

func notDemo(context.Context, uuid.UUID) bool { return false }

func newSharing(members *svcBudgetMemberRepo, invites *svcBudgetInviteRepo) *budgetSharingService {
	svc := NewBudgetSharingService(members, invites, fakeDBTxn{}, notDemo).(*budgetSharingService)
	svc.now = func() time.Time { return time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC) }
	return svc
}

// sharingCtx is a request in budgetID by userID with role.
func sharingCtx(budgetID, userID uuid.UUID, role model.BudgetRole) context.Context {
	ctx := utils.WithUserID(utils.WithBudgetID(context.Background(), budgetID), userID)
	return utils.WithBudgetRole(ctx, role)
}

func TestBudgetSharing_Invite(t *testing.T) {
	budgetID, ownerID := uuid.New(), uuid.New()
	owner := sharingCtx(budgetID, ownerID, model.BudgetRoleOwner)
	ownerOnly := []model.BudgetMember{{UserID: ownerID, Role: model.BudgetRoleOwner}}

	t.Run("owner_invites_lowercased_email", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		members.On("List", mock.Anything, budgetID).Return(ownerOnly, nil)
		members.On("EmailsOfUser", mock.Anything, ownerID).Return([]string{"owner@example.com"}, nil)
		invites.On("Create", mock.Anything, mock.MatchedBy(func(inv model.BudgetInvite) bool {
			return inv.Email == "partner@example.com" && inv.Role == model.BudgetRoleEditor &&
				inv.BudgetID == budgetID && inv.InvitedBy == ownerID &&
				inv.ExpiresAt.Equal(time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC))
		})).Return(&model.BudgetInvite{ID: uuid.New()}, nil)

		_, err := newSharing(members, invites).Invite(owner, model.CreateBudgetInviteRequest{
			Email: "  Partner@Example.com ", Role: model.BudgetRoleEditor,
		})
		assert.NoError(t, err)
		invites.AssertExpectations(t)
	})
	t.Run("editor_cannot_invite", func(t *testing.T) {
		editor := sharingCtx(budgetID, uuid.New(), model.BudgetRoleEditor)
		_, err := newSharing(&svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}).Invite(editor, model.CreateBudgetInviteRequest{
			Email: "x@example.com", Role: model.BudgetRoleViewer,
		})
		assert.Equal(t, http.StatusForbidden, errs.StatusOf(err))
	})
	t.Run("owner_role_is_refused", func(t *testing.T) {
		_, err := newSharing(&svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}).Invite(owner, model.CreateBudgetInviteRequest{
			Email: "x@example.com", Role: model.BudgetRoleOwner,
		})
		assert.Equal(t, http.StatusBadRequest, errs.StatusOf(err))
	})
	t.Run("bad_email_is_refused", func(t *testing.T) {
		_, err := newSharing(&svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}).Invite(owner, model.CreateBudgetInviteRequest{
			Email: "Name <x@example.com>", Role: model.BudgetRoleViewer,
		})
		assert.Equal(t, http.StatusBadRequest, errs.StatusOf(err))
	})
	t.Run("existing_member_is_refused", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		members.On("List", mock.Anything, budgetID).Return(ownerOnly, nil)
		members.On("EmailsOfUser", mock.Anything, ownerID).Return([]string{"owner@example.com"}, nil)
		_, err := newSharing(members, invites).Invite(owner, model.CreateBudgetInviteRequest{
			Email: "owner@example.com", Role: model.BudgetRoleViewer,
		})
		assert.True(t, errs.HasCode(err, errs.CodeBudgetAlreadyMember))
		invites.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})
	t.Run("open_invite_is_a_conflict", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		members.On("List", mock.Anything, budgetID).Return(ownerOnly, nil)
		members.On("EmailsOfUser", mock.Anything, ownerID).Return([]string{}, nil)
		invites.On("Create", mock.Anything, mock.Anything).Return(nil, repository.ErrInviteExists)
		_, err := newSharing(members, invites).Invite(owner, model.CreateBudgetInviteRequest{
			Email: "x@example.com", Role: model.BudgetRoleViewer,
		})
		assert.Equal(t, http.StatusConflict, errs.StatusOf(err))
	})
}

func TestBudgetSharing_AcceptInvite(t *testing.T) {
	inviteID, budgetID, inviterID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	invite := &model.BudgetInvite{
		ID: inviteID, BudgetID: budgetID, Email: "me@example.com", Role: model.BudgetRoleViewer, InvitedBy: inviterID,
	}

	t.Run("adds_member_with_invite_role", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		invites.On("GetOpen", mock.Anything, inviteID).Return(invite, nil)
		members.On("EmailsOfUser", mock.Anything, userID).Return([]string{"me@example.com"}, nil)
		invites.On("MarkAccepted", mock.Anything, inviteID, userID).Return(nil)
		members.On("Add", mock.Anything, budgetID, userID, model.BudgetRoleViewer, &inviterID).Return(true, nil)

		got, err := newSharing(members, invites).AcceptInvite(context.Background(), userID, inviteID)
		assert.NoError(t, err)
		assert.Equal(t, budgetID, got)
		members.AssertExpectations(t)
	})
	t.Run("invite_to_another_email_reads_as_not_found", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		invites.On("GetOpen", mock.Anything, inviteID).Return(invite, nil)
		members.On("EmailsOfUser", mock.Anything, userID).Return([]string{"someone-else@example.com"}, nil)

		_, err := newSharing(members, invites).AcceptInvite(context.Background(), userID, inviteID)
		assert.Equal(t, http.StatusNotFound, errs.StatusOf(err))
		invites.AssertNotCalled(t, "MarkAccepted", mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("closed_invite_is_not_found", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		invites.On("GetOpen", mock.Anything, inviteID).Return(nil, repository.ErrNotFound)
		_, err := newSharing(members, invites).AcceptInvite(context.Background(), userID, inviteID)
		assert.True(t, errs.HasCode(err, errs.CodeBudgetInviteNotFound))
	})
	t.Run("accepted_twice_at_once_fails_the_second", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		invites.On("GetOpen", mock.Anything, inviteID).Return(invite, nil)
		members.On("EmailsOfUser", mock.Anything, userID).Return([]string{"me@example.com"}, nil)
		invites.On("MarkAccepted", mock.Anything, inviteID, userID).Return(repository.ErrNotFound)

		_, err := newSharing(members, invites).AcceptInvite(context.Background(), userID, inviteID)
		assert.True(t, errs.HasCode(err, errs.CodeBudgetInviteNotFound))
		members.AssertNotCalled(t, "Add", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestBudgetSharing_RemoveAndLeave(t *testing.T) {
	budgetID, ownerID, memberID := uuid.New(), uuid.New(), uuid.New()

	t.Run("owner_removes_member_and_their_invites", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		members.On("EmailsOfUser", mock.Anything, memberID).Return([]string{"m@example.com"}, nil)
		members.On("Remove", mock.Anything, budgetID, memberID).Return(nil)
		invites.On("RevokeForEmails", mock.Anything, budgetID, []string{"m@example.com"}).Return(nil)

		ctx := sharingCtx(budgetID, ownerID, model.BudgetRoleOwner)
		assert.NoError(t, newSharing(members, invites).RemoveMember(ctx, memberID))
		invites.AssertExpectations(t)
	})
	t.Run("removing_the_owner_is_not_found", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		members.On("EmailsOfUser", mock.Anything, ownerID).Return([]string{}, nil)
		members.On("Remove", mock.Anything, budgetID, ownerID).Return(repository.ErrNotFound)

		ctx := sharingCtx(budgetID, ownerID, model.BudgetRoleOwner)
		err := newSharing(members, invites).RemoveMember(ctx, ownerID)
		assert.Equal(t, http.StatusNotFound, errs.StatusOf(err))
	})
	t.Run("owner_cannot_leave", func(t *testing.T) {
		members := &svcBudgetMemberRepo{}
		members.On("Role", mock.Anything, budgetID, ownerID).Return(model.BudgetRoleOwner, nil)
		err := newSharing(members, &svcBudgetInviteRepo{}).Leave(context.Background(), ownerID, budgetID)
		assert.True(t, errs.HasCode(err, errs.CodeBudgetOwnerCannotLeave))
	})
	t.Run("viewer_leaves", func(t *testing.T) {
		members, invites := &svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}
		members.On("Role", mock.Anything, budgetID, memberID).Return(model.BudgetRoleViewer, nil)
		members.On("EmailsOfUser", mock.Anything, memberID).Return([]string{}, nil)
		members.On("Remove", mock.Anything, budgetID, memberID).Return(nil)
		invites.On("RevokeForEmails", mock.Anything, budgetID, []string{}).Return(nil)
		assert.NoError(t, newSharing(members, invites).Leave(context.Background(), memberID, budgetID))
		members.AssertExpectations(t)
	})
}

func TestBudgetSharing_ChangeRole(t *testing.T) {
	budgetID, memberID := uuid.New(), uuid.New()
	owner := sharingCtx(budgetID, uuid.New(), model.BudgetRoleOwner)

	t.Run("cannot_make_someone_owner", func(t *testing.T) {
		err := newSharing(&svcBudgetMemberRepo{}, &svcBudgetInviteRepo{}).ChangeRole(owner, memberID, model.BudgetRoleOwner)
		assert.Equal(t, http.StatusBadRequest, errs.StatusOf(err))
	})
	t.Run("owner_changes_role", func(t *testing.T) {
		members := &svcBudgetMemberRepo{}
		members.On("UpdateRole", mock.Anything, budgetID, memberID, model.BudgetRoleEditor).Return(nil)
		assert.NoError(t, newSharing(members, &svcBudgetInviteRepo{}).ChangeRole(owner, memberID, model.BudgetRoleEditor))
	})
}
