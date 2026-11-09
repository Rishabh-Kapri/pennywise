package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
)

func TestMailConnectionRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMailConnectionRepository(testPool(t))
	b1, b2 := uuid.New(), uuid.New()

	// Connect: account is stored lowercase.
	c, err := repo.Upsert(ctx, "Gmail", " Me@Gmail.com ", []byte("sealed-1"), b1)
	if err != nil {
		t.Fatal(err)
	}
	if c.Provider != "gmail" || c.Account != "me@gmail.com" || c.Status != model.MailConnectionActive ||
		len(c.BudgetIDs) != 1 || c.BudgetIDs[0] != b1 || c.Cursor != "" || c.WatchExpiry != nil {
		t.Fatalf("created = %+v", c)
	}

	// Reconnect to another budget, then the same budget again: one row, each
	// budget once, credentials replaced.
	if _, err := repo.Upsert(ctx, "gmail", "me@gmail.com", []byte("sealed-2"), b2); err != nil {
		t.Fatal(err)
	}
	c2, err := repo.Upsert(ctx, "gmail", "ME@gmail.com", []byte("sealed-3"), b2)
	if err != nil {
		t.Fatal(err)
	}
	if c2.ID != c.ID || len(c2.BudgetIDs) != 2 || string(c2.Credentials) != "sealed-3" {
		t.Fatalf("after reconnect = %+v", c2)
	}

	// A push finds it whatever the case.
	byAccount, err := repo.GetByAccount(ctx, "gmail", "Me@GMAIL.com")
	if err != nil || byAccount.ID != c.ID {
		t.Fatalf("GetByAccount = %+v, %v", byAccount, err)
	}
	_, err = repo.GetByAccount(ctx, "gmail", "someone@else.com")
	if !errs.HasCode(err, errs.CodeConnectionNotFound) || !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("unknown account: err = %v", err)
	}

	// Budget listing.
	list, err := repo.ListByBudget(ctx, b2)
	if err != nil || len(list) != 1 || list[0].ID != c.ID {
		t.Fatalf("ListByBudget = %+v, %v", list, err)
	}

	// No watch yet: due for renewal.
	due, err := repo.ListWatchesExpiring(ctx, time.Now())
	if err != nil || len(due) != 1 {
		t.Fatalf("expiring before watch = %+v, %v", due, err)
	}
	expires := time.Now().Add(7 * 24 * time.Hour).Truncate(time.Microsecond)
	if err := repo.SaveWatch(ctx, c.ID, "1000", expires); err != nil {
		t.Fatal(err)
	}
	due, _ = repo.ListWatchesExpiring(ctx, time.Now().Add(24*time.Hour))
	if len(due) != 0 {
		t.Fatalf("watch not due yet, got %d", len(due))
	}
	due, _ = repo.ListWatchesExpiring(ctx, time.Now().Add(8*24*time.Hour))
	if len(due) != 1 {
		t.Fatalf("watch due within 8 days, got %d", len(due))
	}

	// Failure, then a successful sync clears it.
	if err := repo.SaveFailure(ctx, c.ID, model.MailConnectionNeedsReconnect, "token revoked"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(ctx, c.ID)
	if got.Status != model.MailConnectionNeedsReconnect || got.LastError != "token revoked" || got.Cursor != "1000" {
		t.Fatalf("after failure = %+v", got)
	}
	due, _ = repo.ListWatchesExpiring(ctx, time.Now().Add(8*24*time.Hour))
	if len(due) != 0 {
		t.Fatal("needs_reconnect must not be offered for watch renewal")
	}
	if err := repo.SaveSync(ctx, c.ID, "1050"); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetByID(ctx, c.ID)
	if got.Status != model.MailConnectionActive || got.LastError != "" || got.Cursor != "1050" ||
		got.LastSyncAt == nil || !got.WatchExpiry.Equal(expires) {
		t.Fatalf("after sync = %+v", got)
	}

	// A paused connection stays listed but isn't up for renewal.
	paused, err := repo.SetPaused(ctx, c.ID, true)
	if err != nil || paused.PausedAt == nil {
		t.Fatalf("paused = %+v, %v", paused, err)
	}
	due, _ = repo.ListWatchesExpiring(ctx, time.Now().Add(8*24*time.Hour))
	if len(due) != 0 {
		t.Fatal("paused connection offered for renewal")
	}
	resumed, err := repo.SetPaused(ctx, c.ID, false)
	if err != nil || resumed.PausedAt != nil {
		t.Fatalf("resumed = %+v, %v", resumed, err)
	}

	// Detach both budgets: deleted with its credentials, and not up for renewal.
	if _, err := repo.DetachBudget(ctx, c.ID, b1); err != nil {
		t.Fatal(err)
	}
	disabled, err := repo.DetachBudget(ctx, c.ID, b2)
	if err != nil || len(disabled.BudgetIDs) != 0 {
		t.Fatalf("after detaching all = %+v, %v", disabled, err)
	}
	if len(disabled.Credentials) != 0 {
		t.Fatal("credentials kept after the last budget left")
	}
	if _, err := repo.GetByID(ctx, c.ID); !errs.HasCode(err, errs.CodeConnectionNotFound) {
		t.Fatalf("deleted connection: err = %v", err)
	}
	due, _ = repo.ListWatchesExpiring(ctx, time.Now().Add(8*24*time.Hour))
	if len(due) != 0 {
		t.Fatal("disabled connection offered for renewal")
	}

	// Updates on a missing connection say so.
	if err := repo.SaveSync(ctx, uuid.New(), "1"); !errs.HasCode(err, errs.CodeConnectionNotFound) {
		t.Fatalf("missing: err = %v", err)
	}
}
