package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
)

type fakeConns struct {
	repository.MailConnectionRepository
	conn       *model.MailConnection
	savedSync  string
	failStatus model.MailConnectionStatus
}

func (f *fakeConns) GetByAccount(context.Context, string, string) (*model.MailConnection, error) {
	if f.conn == nil {
		return nil, errs.New(errs.CodeConnectionNotFound, "not found")
	}
	return f.conn, nil
}
func (f *fakeConns) SaveSync(_ context.Context, _ uuid.UUID, cursor string) error {
	f.savedSync = cursor
	return nil
}
func (f *fakeConns) SaveFailure(_ context.Context, _ uuid.UUID, s model.MailConnectionStatus, _ string) error {
	f.failStatus = s
	return nil
}

type fakeBudgets struct {
	repository.BudgetRepository
	deleted map[uuid.UUID]bool
}

func (f *fakeBudgets) GetById(_ context.Context, id uuid.UUID) (*model.Budget, error) {
	if f.deleted[id] {
		return nil, repository.ErrNotFound
	}
	return &model.Budget{}, nil
}

type fakeProvider struct {
	ports.MailProvider
	msgs   []ports.InboundMessage
	cursor string
	err    error
}

func (f *fakeProvider) FetchSince(context.Context, model.MailConnection) ([]ports.InboundMessage, string, error) {
	return f.msgs, f.cursor, f.err
}

type oneProvider struct{ p ports.MailProvider }

func (o oneProvider) For(string) (ports.MailProvider, error) { return o.p, nil }

type fakeIngest struct {
	got []accepted
	err error
}

type accepted struct {
	budgets []uuid.UUID
	msg     ports.InboundMessage
}

func (f *fakeIngest) Accept(
	_ context.Context, budgets []uuid.UUID, msg ports.InboundMessage, _ ...job.EnqueueOption,
) (Accepted, error) {
	if f.err != nil {
		return Accepted{}, f.err
	}
	if msg.Body == "" {
		return Accepted{}, errs.New(errs.CodeIngestInvalidMessage, "no body")
	}
	f.got = append(f.got, accepted{budgets: budgets, msg: msg})
	out := Accepted{ExternalID: msg.ExternalID}
	for _, b := range budgets {
		out.Jobs = append(out.Jobs, Queued{BudgetID: b, JobID: uuid.New()})
	}
	return out, nil
}

var push = model.MailboxChanged{Provider: "gmail", Account: "me@gmail.com", Cursor: "200"}

func setupSync(conn *model.MailConnection, p *fakeProvider, deleted ...uuid.UUID) (MailboxService, *fakeConns, *fakeIngest) {
	conns := &fakeConns{conn: conn}
	ing := &fakeIngest{}
	gone := map[uuid.UUID]bool{}
	for _, id := range deleted {
		gone[id] = true
	}
	s := NewMailboxService(MailboxDeps{
		Connections: conns, Budgets: &fakeBudgets{deleted: gone}, Providers: oneProvider{p}, Ingest: ing,
	})
	return s, conns, ing
}

func TestSyncQueuesForLiveBudgetsThenMovesCursor(t *testing.T) {
	live, gone := uuid.New(), uuid.New()
	conn := &model.MailConnection{ID: uuid.New(), Provider: "gmail", BudgetIDs: []uuid.UUID{live, gone}, Cursor: "100"}
	p := &fakeProvider{cursor: "180", msgs: []ports.InboundMessage{
		{ExternalID: "m1", Body: "Rs 499 debited"},
		{ExternalID: "m2", Body: ""}, // can never be queued
	}}
	s, conns, ing := setupSync(conn, p, gone)

	res, err := s.Sync(context.Background(), push)
	if err != nil {
		t.Fatal(err)
	}
	if res.Fetched != 2 || res.Queued != 1 || res.Dropped != 1 || res.Cursor != "180" || res.Skipped != "" {
		t.Fatalf("result = %+v", res)
	}
	if len(ing.got) != 1 || len(ing.got[0].budgets) != 1 || ing.got[0].budgets[0] != live {
		t.Fatalf("queued for %+v, want only the live budget", ing.got)
	}
	if conns.savedSync != "180" {
		t.Fatalf("cursor saved = %q", conns.savedSync)
	}
}

func TestSyncDoesNotMoveCursorWhenQueueingFails(t *testing.T) {
	conn := &model.MailConnection{ID: uuid.New(), BudgetIDs: []uuid.UUID{uuid.New()}}
	p := &fakeProvider{cursor: "180", msgs: []ports.InboundMessage{{ExternalID: "m1", Body: "b"}}}
	s, conns, ing := setupSync(conn, p)
	ing.err = errors.New("db down")

	if _, err := s.Sync(context.Background(), push); err == nil {
		t.Fatal("want an error so the job retries")
	}
	if conns.savedSync != "" {
		t.Fatal("cursor moved although messages weren't queued")
	}
}

func TestSyncSkipsWhatRetryingCantFix(t *testing.T) {
	budget := []uuid.UUID{uuid.New()}
	cases := map[string]struct {
		conn       *model.MailConnection
		fetchErr   error
		wantStatus model.MailConnectionStatus
	}{
		"unknown mailbox": {conn: nil},
		"disabled":        {conn: &model.MailConnection{ID: uuid.New(), BudgetIDs: []uuid.UUID{}}},
		"needs reconnect": {conn: &model.MailConnection{ID: uuid.New(), BudgetIDs: budget,
			Status: model.MailConnectionNeedsReconnect}},
		"token refused": {conn: &model.MailConnection{ID: uuid.New(), BudgetIDs: budget},
			fetchErr: errs.New(errs.CodeMailAuthFailed, "revoked"), wantStatus: model.MailConnectionNeedsReconnect},
		"cursor expired": {conn: &model.MailConnection{ID: uuid.New(), BudgetIDs: budget},
			fetchErr: errs.New(errs.CodeMailCursorExpired, "404"), wantStatus: model.MailConnectionError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s, conns, _ := setupSync(tc.conn, &fakeProvider{err: tc.fetchErr})
			res, err := s.Sync(context.Background(), push)
			if err != nil || res.Skipped == "" {
				t.Fatalf("got (%+v, %v), want a skip and no error", res, err)
			}
			if conns.failStatus != tc.wantStatus {
				t.Fatalf("recorded status %q, want %q", conns.failStatus, tc.wantStatus)
			}
		})
	}
}

func TestSyncSkipsPausedConnection(t *testing.T) {
	now := time.Now()
	conn := &model.MailConnection{
		ID: uuid.New(), Provider: "gmail", Account: "me@gmail.com",
		BudgetIDs: []uuid.UUID{uuid.New()}, Status: model.MailConnectionActive, PausedAt: &now,
	}
	s, conns, _ := setupSync(conn, &fakeProvider{})
	res, err := s.Sync(context.Background(), model.MailboxChanged{Provider: "gmail", Account: "me@gmail.com"})
	if err != nil || res.Skipped == "" || conns.savedSync != "" {
		t.Fatalf("res = %+v, saved = %q, err = %v", res, conns.savedSync, err)
	}
}

func TestSyncRetriesTransientFetchErrors(t *testing.T) {
	conn := &model.MailConnection{ID: uuid.New(), BudgetIDs: []uuid.UUID{uuid.New()}}
	s, conns, _ := setupSync(conn, &fakeProvider{err: errs.NewWith(errs.CodeMailRequestFailed, "503", errs.WithStatus(503))})
	if _, err := s.Sync(context.Background(), push); err == nil {
		t.Fatal("want an error so the job retries")
	}
	if conns.failStatus != model.MailConnectionError {
		t.Fatalf("recorded status %q", conns.failStatus)
	}
}
