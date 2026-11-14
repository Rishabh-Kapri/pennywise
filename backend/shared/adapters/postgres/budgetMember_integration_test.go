package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newLoginUser creates an auth user with a Google login named name/email.
func newLoginUser(t *testing.T, pool *pgxpool.Pool, name, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	googleID := "g-" + userID.String()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO auth_users (id) VALUES ($1)`, []any{userID}},
		{`INSERT INTO auth_providers (auth_user_id, provider_type, provider_id) VALUES ($1, 'google', $2)`,
			[]any{userID, googleID}},
		{`INSERT INTO google_provider_users (id, name, email) VALUES ($1, $2, $3)`, []any{googleID, name, email}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	return userID
}

func TestBudgetMembersAndInvites(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	budgets := NewBudgetRepository(pool)
	members := NewBudgetMemberRepository(pool)
	invites := NewBudgetInviteRepository(pool)

	suffix := uuid.NewString()[:8]
	ownerEmail, partnerEmail := "owner-"+suffix+"@example.com", "partner-"+suffix+"@example.com"
	ownerID := newLoginUser(t, pool, "Owner", ownerEmail)
	partnerID := newLoginUser(t, pool, "Partner", "Partner-"+suffix+"@Example.com")

	first, err := budgets.Create(ctx, "Home", ownerID)
	if err != nil {
		t.Fatalf("create budget: %v", err)
	}
	second, err := budgets.Create(ctx, "Side", ownerID)
	if err != nil {
		t.Fatalf("create second budget: %v", err)
	}
	if !first.IsSelected || second.IsSelected {
		t.Fatalf("only the first budget should open selected: first %v, second %v", first.IsSelected, second.IsSelected)
	}
	if first.Role != model.BudgetRoleOwner {
		t.Fatalf("creator's role = %q, want owner", first.Role)
	}

	t.Run("non_member_has_no_role", func(t *testing.T) {
		if _, err := members.Role(ctx, first.ID, partnerID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("Role for a non-member: %v, want ErrNotFound", err)
		}
	})

	var invite *model.BudgetInvite
	t.Run("invite_and_find_by_email", func(t *testing.T) {
		invite, err = invites.Create(ctx, model.BudgetInvite{
			BudgetID: first.ID, Email: partnerEmail, Role: model.BudgetRoleEditor,
			InvitedBy: ownerID, ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("create invite: %v", err)
		}
		_, err := invites.Create(ctx, model.BudgetInvite{
			BudgetID: first.ID, Email: partnerEmail, Role: model.BudgetRoleViewer,
			InvitedBy: ownerID, ExpiresAt: time.Now().Add(time.Hour),
		})
		if !errors.Is(err, repository.ErrInviteExists) {
			t.Fatalf("second open invite: %v, want ErrInviteExists", err)
		}

		emails, err := members.EmailsOfUser(ctx, partnerID)
		if err != nil || len(emails) != 1 || emails[0] != partnerEmail {
			t.Fatalf("EmailsOfUser = %v, %v; want the lowercase login email", emails, err)
		}
		mine, err := invites.ListForEmails(ctx, emails)
		if err != nil || len(mine) != 1 {
			t.Fatalf("ListForEmails = %v, %v; want one invite", mine, err)
		}
		if mine[0].BudgetName != "Home" || mine[0].InviterName != "Owner" {
			t.Fatalf("invite shows budget %q from %q, want Home from Owner", mine[0].BudgetName, mine[0].InviterName)
		}
	})

	t.Run("accept_adds_member_once", func(t *testing.T) {
		if err := invites.MarkAccepted(ctx, invite.ID, partnerID); err != nil {
			t.Fatalf("accept: %v", err)
		}
		if err := invites.MarkAccepted(ctx, invite.ID, partnerID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("accept twice: %v, want ErrNotFound", err)
		}
		added, err := members.Add(ctx, first.ID, partnerID, model.BudgetRoleEditor, &ownerID)
		if err != nil || !added {
			t.Fatalf("Add = %v, %v", added, err)
		}
		again, err := members.Add(ctx, first.ID, partnerID, model.BudgetRoleViewer, &ownerID)
		if err != nil || again {
			t.Fatalf("Add again = %v, %v; want false, nil", again, err)
		}
		role, err := members.Role(ctx, first.ID, partnerID)
		if err != nil || role != model.BudgetRoleEditor {
			t.Fatalf("Role = %q, %v; want editor (a second Add changes nothing)", role, err)
		}
	})

	t.Run("list_members_owner_first", func(t *testing.T) {
		list, err := members.List(ctx, first.ID)
		if err != nil || len(list) != 2 {
			t.Fatalf("List = %v, %v", list, err)
		}
		if list[0].UserID != ownerID || list[0].Name != "Owner" || list[1].Email != partnerEmail {
			t.Fatalf("List = %+v, want owner first and the partner's lowercase email", list)
		}
	})

	t.Run("selection_is_per_member", func(t *testing.T) {
		partnerBudgets, err := budgets.GetAll(ctx, partnerID)
		if err != nil || len(partnerBudgets) != 1 {
			t.Fatalf("partner GetAll = %v, %v", partnerBudgets, err)
		}
		if partnerBudgets[0].IsSelected || partnerBudgets[0].MemberCount != 2 {
			t.Fatalf("partner's budget = %+v, want unselected with 2 members", partnerBudgets[0])
		}
		if err := members.SetSelected(ctx, partnerID, first.ID); err != nil {
			t.Fatalf("SetSelected: %v", err)
		}
		if err := members.SetSelected(ctx, partnerID, second.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("SetSelected on a budget they're not in: %v, want ErrNotFound", err)
		}
		if err := members.SetSelected(ctx, ownerID, second.ID); err != nil {
			t.Fatalf("owner SetSelected: %v", err)
		}
		ownerBudgets, _ := budgets.GetAll(ctx, ownerID)
		selected := map[uuid.UUID]bool{}
		for _, b := range ownerBudgets {
			selected[b.ID] = b.IsSelected
		}
		if selected[first.ID] || !selected[second.ID] {
			t.Fatalf("owner's selection = %v, want only the second budget", selected)
		}
		partnerBudgets, _ = budgets.GetAll(ctx, partnerID)
		if !partnerBudgets[0].IsSelected {
			t.Fatal("the owner switching budgets moved the partner's selection")
		}
	})

	t.Run("owner_row_is_fixed", func(t *testing.T) {
		if err := members.UpdateRole(ctx, first.ID, ownerID, model.BudgetRoleViewer); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("demote owner: %v, want ErrNotFound", err)
		}
		if err := members.Remove(ctx, first.ID, ownerID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("remove owner: %v, want ErrNotFound", err)
		}
	})

	t.Run("remove_member_and_reinvite_after_expiry", func(t *testing.T) {
		if err := members.Remove(ctx, first.ID, partnerID); err != nil {
			t.Fatalf("remove: %v", err)
		}
		if _, err := members.Role(ctx, first.ID, partnerID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("removed member still has a role: %v", err)
		}
		expired, err := invites.Create(ctx, model.BudgetInvite{
			BudgetID: first.ID, Email: partnerEmail, Role: model.BudgetRoleViewer,
			InvitedBy: ownerID, ExpiresAt: time.Now().Add(-time.Minute),
		})
		if err != nil {
			t.Fatalf("create expired invite: %v", err)
		}
		if _, err := invites.GetOpen(ctx, expired.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("expired invite is open: %v", err)
		}
		if _, err := invites.Create(ctx, model.BudgetInvite{
			BudgetID: first.ID, Email: partnerEmail, Role: model.BudgetRoleViewer,
			InvitedBy: ownerID, ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("an expired invite blocked a new one: %v", err)
		}
		if err := invites.RevokeForEmails(ctx, first.ID, []string{partnerEmail}); err != nil {
			t.Fatalf("RevokeForEmails: %v", err)
		}
		open, err := invites.ListByBudget(ctx, first.ID)
		if err != nil || len(open) != 0 {
			t.Fatalf("ListByBudget after revoke = %v, %v; want none", open, err)
		}
	})

	t.Run("push_tokens_reach_every_member", func(t *testing.T) {
		if _, err := members.Add(ctx, first.ID, partnerID, model.BudgetRoleViewer, &ownerID); err != nil {
			t.Fatalf("re-add: %v", err)
		}
		tokens := NewDevicePushTokenRepository(pool)
		for _, u := range []uuid.UUID{ownerID, partnerID} {
			if _, err := tokens.Upsert(ctx, u, "ExponentPushToken["+u.String()+"]", "android"); err != nil {
				t.Fatalf("upsert token: %v", err)
			}
		}
		got, err := tokens.GetByBudgetId(ctx, first.ID)
		if err != nil || len(got) != 2 {
			t.Fatalf("GetByBudgetId = %d tokens, %v; want 2", len(got), err)
		}
	})
}
