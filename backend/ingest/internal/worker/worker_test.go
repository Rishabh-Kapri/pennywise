package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

type fakeMailbox struct {
	service.MailboxService
	opts   []job.EnqueueOption
	budget uuid.UUID
	ids    []string
	err    error
}

func (f *fakeMailbox) Sync(
	_ context.Context, _ model.MailboxChanged, opts ...job.EnqueueOption,
) (service.SyncResult, error) {
	f.opts = opts
	return service.SyncResult{Counts: service.Counts{Queued: 2}}, f.err
}

func (f *fakeMailbox) Backfill(
	_ context.Context, budget, _ uuid.UUID, ids []string, opts ...job.EnqueueOption,
) (service.BackfillResult, error) {
	f.budget, f.ids, f.opts = budget, ids, opts
	return service.BackfillResult{Counts: service.Counts{Queued: len(ids)}}, f.err
}

func payload(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func family(t *testing.T, opts []job.EnqueueOption) (root, parent uuid.UUID) {
	t.Helper()
	o, err := job.ResolveEnqueueOptions("t", []byte(`{}`), append(opts, job.WithQueue("q"))...)
	if err != nil {
		t.Fatal(err)
	}
	if o.RootJobID == nil || o.ParentID == nil {
		t.Fatalf("no family in %+v", o)
	}
	return *o.RootJobID, *o.ParentID
}

func TestMailboxChangedSyncsUnderItself(t *testing.T) {
	m := &fakeMailbox{}
	j := &job.Job{ID: uuid.New(), Payload: payload(t, model.MailboxChanged{Provider: "gmail", Account: "a@b.c"})}

	result, err := mailboxChanged(Deps{Mailbox: m})(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	var got service.SyncResult
	if err := json.Unmarshal(result, &got); err != nil || got.Queued != 2 {
		t.Fatalf("result %s: %v", result, err)
	}
	if root, parent := family(t, m.opts); root != j.ID || parent != j.ID {
		t.Fatalf("root %s parent %s", root, parent)
	}
}

func TestMailboxChangedRetriesSyncFailures(t *testing.T) {
	boom := errors.New("gmail down")
	j := &job.Job{ID: uuid.New(), Payload: payload(t, model.MailboxChanged{})}
	if _, err := mailboxChanged(Deps{Mailbox: &fakeMailbox{err: boom}})(context.Background(), j); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}

func TestMailboxBackfillImportsUnderItself(t *testing.T) {
	budget := uuid.New()
	m := &fakeMailbox{}
	j := &job.Job{
		ID: uuid.New(), BudgetID: &budget,
		Payload: payload(t, model.MailboxBackfill{ConnectionID: uuid.New(), ExternalIDs: []string{"a", "b"}}),
	}

	if _, err := mailboxBackfill(Deps{Mailbox: m})(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if m.budget != budget || len(m.ids) != 2 {
		t.Fatalf("%+v", m)
	}
	if root, parent := family(t, m.opts); root != j.ID || parent != j.ID {
		t.Fatalf("root %s parent %s", root, parent)
	}
}

func TestMailboxBackfillNeedsABudget(t *testing.T) {
	j := &job.Job{ID: uuid.New(), Payload: payload(t, model.MailboxBackfill{})}
	if _, err := mailboxBackfill(Deps{Mailbox: &fakeMailbox{}})(context.Background(), j); !errs.HasCode(err, errs.CodeJobInvalidPayload) {
		t.Fatalf("got %v", err)
	}
}
