package service

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
)

func TestMailboxChangedDedupesRedeliveries(t *testing.T) {
	broker := &fakeBroker{byKey: map[string]uuid.UUID{}}
	s := NewMailboxService(MailboxDeps{Queue: broker})
	n := model.MailboxChanged{Provider: "Gmail", Account: " Me@Gmail.com ", Cursor: "100"}

	first, err := s.Changed(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Changed(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Duplicate || again.JobID != first.JobID {
		t.Fatalf("redelivery made a new job: %+v vs %+v", again, first)
	}
	if key := broker.calls[0].IdempotencyKey; key != "gmail:me@gmail.com:100" {
		t.Fatalf("key = %q", key)
	}
	if broker.calls[0].Type != model.JobTypeMailboxChanged {
		t.Fatalf("type = %q", broker.calls[0].Type)
	}
}

func TestMailboxChangedRejects(t *testing.T) {
	_, err := NewMailboxService(MailboxDeps{Queue: &fakeBroker{byKey: map[string]uuid.UUID{}}}).
		Changed(context.Background(), model.MailboxChanged{Provider: "gmail", Account: "a@b.c"})
	if !errs.HasCode(err, errs.CodeIngestInvalidMessage) {
		t.Fatalf("missing cursor: err = %v", err)
	}
	_, err = NewMailboxService(MailboxDeps{Queue: NewEnqueuer(nil)}).Changed(context.Background(), model.MailboxChanged{Provider: "gmail", Account: "a@b.c", Cursor: "1"})
	if !errs.HasCode(err, errs.CodeIngestQueueUnavailable) {
		t.Fatalf("no queue: err = %v", err)
	}
}

func TestMailboxTest(t *testing.T) {
	budget := uuid.New()
	conn := &model.MailConnection{Provider: "gmail", Account: "me@example.com", BudgetIDs: []uuid.UUID{budget}}
	broker := &fakeBroker{byKey: map[string]uuid.UUID{}}
	s := NewMailboxService(MailboxDeps{Queue: broker, Connections: &fakeConns{conn: conn}})
	ctx := context.Background()

	first, err := s.Test(ctx, budget, "gmail", conn.Account)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Test(ctx, budget, "gmail", conn.Account)
	if err != nil {
		t.Fatal(err)
	}
	if second.Duplicate || second.JobID == first.JobID {
		t.Fatalf("repeat was deduped: %+v vs %+v", second, first)
	}
	if broker.calls[0].Queue != model.QueueIngest || broker.calls[0].Type != model.JobTypeMailboxChanged {
		t.Fatalf("%+v", broker.calls[0])
	}

	_, err = s.Test(ctx, uuid.New(), "gmail", conn.Account)
	if !errs.HasCode(err, errs.CodeConnectionNotFound) {
		t.Fatalf("other budget: %v", err)
	}
	if len(broker.calls) != 2 {
		t.Fatalf("queued for another budget: %d jobs", len(broker.calls))
	}
}

type connectConns struct {
	fakeConns
	saved      []byte
	cursor     string
	failStatus model.MailConnectionStatus
}

func (c *connectConns) Upsert(
	_ context.Context, provider, account string, creds []byte, budget uuid.UUID,
) (*model.MailConnection, error) {
	c.saved = creds
	return &model.MailConnection{ID: uuid.New(), Provider: provider, Account: account, BudgetIDs: []uuid.UUID{budget}}, nil
}

func (c *connectConns) SaveWatch(_ context.Context, _ uuid.UUID, cursor string, _ time.Time) error {
	c.cursor = cursor
	return nil
}

func (c *connectConns) SaveFailure(_ context.Context, _ uuid.UUID, s model.MailConnectionStatus, _ string) error {
	c.failStatus = s
	return nil
}

type connectProvider struct {
	ports.MailProvider
	connectErr error
	watchErr   error
}

func (p *connectProvider) Connect(context.Context, string) (ports.ConnectedMailbox, error) {
	return ports.ConnectedMailbox{Account: "me@gmail.com", Credentials: []byte("sealed")}, p.connectErr
}

func (p *connectProvider) Watch(context.Context, model.MailConnection) (string, time.Time, error) {
	return "42", time.Now().Add(time.Hour), p.watchErr
}

func TestConnect(t *testing.T) {
	ctx := context.Background()
	budget := uuid.New()
	connect := func(p *connectProvider, conns *connectConns) (*model.MailConnection, error) {
		s := NewMailboxService(MailboxDeps{Connections: conns, Providers: oneProvider{p}})
		return s.Connect(ctx, budget, "gmail", "code")
	}

	t.Run("saves the mailbox and its watch", func(t *testing.T) {
		conns := &connectConns{}
		conn, err := connect(&connectProvider{}, conns)
		if err != nil {
			t.Fatal(err)
		}
		if string(conns.saved) != "sealed" || conns.cursor != "42" || conn.Cursor != "42" || conn.Account != "me@gmail.com" {
			t.Fatalf("%+v %+v", conns, conn)
		}
	})

	t.Run("a refused code saves nothing", func(t *testing.T) {
		conns := &connectConns{}
		boom := errors.New("refused")
		if _, err := connect(&connectProvider{connectErr: boom}, conns); !errors.Is(err, boom) {
			t.Fatalf("got %v", err)
		}
		if conns.saved != nil {
			t.Fatal("saved a mailbox")
		}
	})

	t.Run("a failed watch is recorded on the connection", func(t *testing.T) {
		conns := &connectConns{}
		boom := errors.New("no pubsub")
		if _, err := connect(&connectProvider{watchErr: boom}, conns); !errors.Is(err, boom) {
			t.Fatalf("got %v", err)
		}
		if conns.failStatus != model.MailConnectionError || conns.cursor != "" {
			t.Fatalf("%+v", conns)
		}
	})
}

type backfillConns struct {
	fakeConns
	conn *model.MailConnection
}

func (c *backfillConns) GetByID(context.Context, uuid.UUID) (*model.MailConnection, error) {
	return c.conn, nil
}

type backfillProvider struct {
	ports.MailProvider
	query ports.Query
	msgs  []ports.InboundMessage
}

func (p *backfillProvider) Search(_ context.Context, _ model.MailConnection, q ports.Query) ([]ports.InboundMessage, error) {
	p.query = q
	return p.msgs, nil
}

func (p *backfillProvider) Fetch(_ context.Context, _ model.MailConnection, ids []string) ([]ports.InboundMessage, error) {
	var out []ports.InboundMessage
	for _, m := range p.msgs {
		if slices.Contains(ids, m.ExternalID) {
			out = append(out, m)
		}
	}
	return out, nil
}

func TestQueueBackfill(t *testing.T) {
	ctx := context.Background()
	budget := uuid.New()
	conns := &backfillConns{conn: &model.MailConnection{
		ID: uuid.New(), Provider: "gmail", Account: "me@example.com", BudgetIDs: []uuid.UUID{budget},
	}}
	broker := &fakeBroker{byKey: map[string]uuid.UUID{}}
	s := NewMailboxService(MailboxDeps{Queue: broker, Connections: conns})

	first, err := s.QueueBackfill(ctx, budget, uuid.New(), []string{"b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.QueueBackfill(ctx, budget, uuid.New(), []string{"a", "b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if !again.Duplicate || again.JobID != first.JobID {
		t.Fatalf("same choice made a new job: %+v vs %+v", again, first)
	}
	if broker.calls[0].Queue != model.QueueIngest || broker.calls[0].Type != model.JobTypeMailboxBackfill {
		t.Fatalf("%+v", broker.calls[0])
	}

	if _, err := s.QueueBackfill(ctx, uuid.New(), uuid.New(), []string{"a"}); !errs.HasCode(err, errs.CodeConnectionNotFound) {
		t.Fatalf("got %v", err)
	}
	tooMany := make([]string, backfillMaxEmails+1)
	for i := range tooMany {
		tooMany[i] = strconv.Itoa(i)
	}
	if _, err := s.QueueBackfill(ctx, budget, uuid.New(), tooMany); !errs.HasCode(err, errs.CodeIngestInvalidMessage) {
		t.Fatalf("more than the candidates: %v", err)
	}
}

func TestBackfill(t *testing.T) {
	ctx := context.Background()
	budget := uuid.New()
	setup := func() (MailboxService, *fakeIngest, *backfillProvider) {
		ingest := &fakeIngest{}
		provider := &backfillProvider{msgs: []ports.InboundMessage{
			{ExternalID: "a", Body: "Rs 10 debited"},
			{ExternalID: "b", Body: "Rs 20 debited"},
		}}
		conns := &backfillConns{conn: &model.MailConnection{Provider: "gmail", BudgetIDs: []uuid.UUID{budget}}}
		return NewMailboxService(MailboxDeps{
			Connections: conns, Providers: oneProvider{provider}, Ingest: ingest,
		}), ingest, provider
	}

	t.Run("candidates queue nothing and are bounded", func(t *testing.T) {
		s, ingest, provider := setup()
		got, err := s.BackfillCandidates(ctx, budget, uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || len(ingest.got) != 0 {
			t.Fatalf("candidates %d, queued %d", len(got), len(ingest.got))
		}
		if provider.query.Days != backfillDays || provider.query.MaxResults != backfillMaxEmails {
			t.Fatalf("%+v", provider.query)
		}
	})

	t.Run("queues only what was chosen, for this budget", func(t *testing.T) {
		s, ingest, _ := setup()
		res, err := s.Backfill(ctx, budget, uuid.New(), []string{"b", "not-a-candidate"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Queued != 1 || len(ingest.got) != 1 || ingest.got[0].msg.ExternalID != "b" {
			t.Fatalf("%+v %+v", res, ingest.got)
		}
		if got := ingest.got[0].budgets; len(got) != 1 || got[0] != budget {
			t.Fatalf("budgets %v", got)
		}
	})

	t.Run("another budget's mailbox reads as missing", func(t *testing.T) {
		s, ingest, _ := setup()
		_, err := s.Backfill(ctx, uuid.New(), uuid.New(), []string{"a"})
		if !errs.HasCode(err, errs.CodeConnectionNotFound) || len(ingest.got) != 0 {
			t.Fatalf("got %v, queued %d", err, len(ingest.got))
		}
		if _, err := s.BackfillCandidates(ctx, uuid.New(), uuid.New()); !errs.HasCode(err, errs.CodeConnectionNotFound) {
			t.Fatalf("got %v", err)
		}
	})
}

type pauseConns struct {
	fakeConns
	conn      *model.MailConnection
	remaining []uuid.UUID // budgets left after DetachBudget
	paused    []bool
	cursor    string
}

func (c *pauseConns) GetByID(context.Context, uuid.UUID) (*model.MailConnection, error) {
	return c.conn, nil
}

func (c *pauseConns) DetachBudget(context.Context, uuid.UUID, uuid.UUID) (*model.MailConnection, error) {
	return &model.MailConnection{ID: c.conn.ID, BudgetIDs: c.remaining}, nil
}

func (c *pauseConns) SetPaused(_ context.Context, _ uuid.UUID, paused bool) (*model.MailConnection, error) {
	c.paused = append(c.paused, paused)
	updated := *c.conn
	updated.PausedAt = nil
	if paused {
		now := time.Now()
		updated.PausedAt = &now
	}
	updated.Status = c.failStatusOr(updated.Status)
	return &updated, nil
}

func (c *pauseConns) failStatusOr(status model.MailConnectionStatus) model.MailConnectionStatus {
	if c.failStatus != "" {
		return c.failStatus
	}
	return status
}

func (c *pauseConns) SaveWatch(_ context.Context, _ uuid.UUID, cursor string, _ time.Time) error {
	c.cursor = cursor
	return nil
}

func (c *pauseConns) SaveFailure(_ context.Context, _ uuid.UUID, s model.MailConnectionStatus, _ string) error {
	c.failStatus = s
	return nil
}

type pauseProvider struct {
	ports.MailProvider
	stopped  int
	stopErr  error
	watchErr error
}

func (p *pauseProvider) StopWatch(context.Context, model.MailConnection) error {
	p.stopped++
	return p.stopErr
}

func (p *pauseProvider) Watch(context.Context, model.MailConnection) (string, time.Time, error) {
	return "99", time.Now().Add(time.Hour), p.watchErr
}

func TestSetPaused(t *testing.T) {
	ctx := context.Background()
	budget := uuid.New()
	now := time.Now()
	setup := func(p *pauseProvider, pausedAt *time.Time) (MailboxService, *pauseConns) {
		conns := &pauseConns{conn: &model.MailConnection{
			ID: uuid.New(), Provider: "gmail", BudgetIDs: []uuid.UUID{budget}, PausedAt: pausedAt,
		}}
		return NewMailboxService(MailboxDeps{Connections: conns, Providers: oneProvider{p}}), conns
	}

	t.Run("pausing stops the watch", func(t *testing.T) {
		p := &pauseProvider{}
		s, conns := setup(p, nil)
		conn, err := s.SetPaused(ctx, budget, conns.conn.ID, true)
		if err != nil || conn.PausedAt == nil || p.stopped != 1 {
			t.Fatalf("conn = %+v, stopped = %d, err = %v", conn, p.stopped, err)
		}
	})

	t.Run("a failed stop still pauses", func(t *testing.T) {
		s, conns := setup(&pauseProvider{stopErr: errors.New("revoked")}, nil)
		conn, err := s.SetPaused(ctx, budget, conns.conn.ID, true)
		if err != nil || conn.PausedAt == nil {
			t.Fatalf("conn = %+v, err = %v", conn, err)
		}
	})

	t.Run("resuming starts a new watch from now", func(t *testing.T) {
		s, conns := setup(&pauseProvider{}, &now)
		conn, err := s.SetPaused(ctx, budget, conns.conn.ID, false)
		if err != nil || conn.PausedAt != nil || conns.cursor != "99" {
			t.Fatalf("conn = %+v, cursor = %q, err = %v", conn, conns.cursor, err)
		}
	})

	t.Run("resuming with ended access asks for a reconnect", func(t *testing.T) {
		s, conns := setup(&pauseProvider{watchErr: errs.New(errs.CodeMailAuthFailed, "revoked")}, &now)
		conn, err := s.SetPaused(ctx, budget, conns.conn.ID, false)
		if err != nil || conn.PausedAt != nil || conn.Status != model.MailConnectionNeedsReconnect {
			t.Fatalf("conn = %+v, err = %v", conn, err)
		}
	})

	t.Run("a transient failure stays paused", func(t *testing.T) {
		s, conns := setup(&pauseProvider{watchErr: errors.New("google is down")}, &now)
		if _, err := s.SetPaused(ctx, budget, conns.conn.ID, false); err == nil || len(conns.paused) != 0 {
			t.Fatalf("paused calls = %v, err = %v", conns.paused, err)
		}
	})

	t.Run("resuming a running mailbox keeps its cursor", func(t *testing.T) {
		s, conns := setup(&pauseProvider{}, nil)
		if _, err := s.SetPaused(ctx, budget, conns.conn.ID, false); err != nil || conns.cursor != "" {
			t.Fatalf("cursor = %q, err = %v", conns.cursor, err)
		}
	})

	t.Run("another budget's mailbox reads as missing", func(t *testing.T) {
		s, conns := setup(&pauseProvider{}, nil)
		_, err := s.SetPaused(ctx, uuid.New(), conns.conn.ID, true)
		if !errs.HasCode(err, errs.CodeConnectionNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestDisconnect(t *testing.T) {
	ctx := context.Background()
	budget := uuid.New()
	setup := func(p *pauseProvider, remaining []uuid.UUID) (MailboxService, *pauseConns) {
		conns := &pauseConns{
			conn:      &model.MailConnection{ID: uuid.New(), Provider: "gmail", BudgetIDs: []uuid.UUID{budget}},
			remaining: remaining,
		}
		return NewMailboxService(MailboxDeps{Connections: conns, Providers: oneProvider{p}}), conns
	}

	t.Run("the last budget stops the watch", func(t *testing.T) {
		p := &pauseProvider{}
		s, conns := setup(p, nil)
		if err := s.Disconnect(ctx, budget, conns.conn.ID); err != nil || p.stopped != 1 {
			t.Fatalf("stopped = %d, err = %v", p.stopped, err)
		}
	})

	t.Run("a mailbox other budgets use keeps its watch", func(t *testing.T) {
		p := &pauseProvider{}
		s, conns := setup(p, []uuid.UUID{uuid.New()})
		if err := s.Disconnect(ctx, budget, conns.conn.ID); err != nil || p.stopped != 0 {
			t.Fatalf("stopped = %d, err = %v", p.stopped, err)
		}
	})

	t.Run("a failed stop still disconnects", func(t *testing.T) {
		s, conns := setup(&pauseProvider{stopErr: errors.New("revoked")}, nil)
		if err := s.Disconnect(ctx, budget, conns.conn.ID); err != nil {
			t.Fatal(err)
		}
	})
}

type fakeSchedules struct {
	job.ScheduleStore
	ensured  []job.JobSchedule
	disabled []string
	err      error
}

func (f *fakeSchedules) EnsureSchedule(_ context.Context, s job.JobSchedule) error {
	f.ensured = append(f.ensured, s)
	return f.err
}

func (f *fakeSchedules) DisableSchedule(_ context.Context, name string) error {
	f.disabled = append(f.disabled, name)
	return nil
}

func TestPauseAndResumeManageTheWatchSchedule(t *testing.T) {
	ctx := context.Background()
	budget := uuid.New()
	now := time.Now()
	setup := func(pausedAt *time.Time, schedules *fakeSchedules) (MailboxService, *pauseConns) {
		conns := &pauseConns{conn: &model.MailConnection{
			ID: uuid.New(), Provider: "gmail", BudgetIDs: []uuid.UUID{budget}, PausedAt: pausedAt,
		}}
		return NewMailboxService(MailboxDeps{
			Connections: conns, Providers: oneProvider{&pauseProvider{}}, Schedules: schedules,
		}), conns
	}

	t.Run("pausing disables it", func(t *testing.T) {
		schedules := &fakeSchedules{}
		s, conns := setup(nil, schedules)
		if _, err := s.SetPaused(ctx, budget, conns.conn.ID, true); err != nil {
			t.Fatal(err)
		}
		if len(schedules.disabled) != 1 || schedules.disabled[0] != model.MailboxWatchScheduleName(conns.conn.ID) {
			t.Fatalf("disabled %v", schedules.disabled)
		}
	})

	t.Run("resuming ensures it", func(t *testing.T) {
		schedules := &fakeSchedules{}
		s, conns := setup(&now, schedules)
		if _, err := s.SetPaused(ctx, budget, conns.conn.ID, false); err != nil {
			t.Fatal(err)
		}
		if len(schedules.ensured) != 1 {
			t.Fatalf("ensured %v", schedules.ensured)
		}
		got := schedules.ensured[0]
		if got.Name != model.MailboxWatchScheduleName(conns.conn.ID) || got.Type != model.JobTypeMailboxStartWatch ||
			got.Queue != model.QueueIngest || !got.NextRunAt.After(now) {
			t.Fatalf("schedule %+v", got)
		}
	})

	t.Run("a failed schedule leaves it paused so resume can be retried", func(t *testing.T) {
		s, conns := setup(&now, &fakeSchedules{err: errors.New("down")})
		if _, err := s.SetPaused(ctx, budget, conns.conn.ID, false); err == nil {
			t.Fatal("want an error")
		}
		if conns.conn.PausedAt == nil {
			t.Fatal("the connection was unpaused")
		}
	})
}

type renewConns struct {
	repository.MailConnectionRepository
	conn       *model.MailConnection
	err        error
	savedCur   string
	savedUntil time.Time
	status     model.MailConnectionStatus
}

func (c *renewConns) GetByID(context.Context, uuid.UUID) (*model.MailConnection, error) {
	return c.conn, c.err
}

func (c *renewConns) SaveWatch(_ context.Context, _ uuid.UUID, cursor string, until time.Time) error {
	c.savedCur, c.savedUntil = cursor, until
	return nil
}

func (c *renewConns) SaveFailure(_ context.Context, _ uuid.UUID, s model.MailConnectionStatus, _ string) error {
	c.status = s
	return nil
}

type renewProvider struct {
	ports.MailProvider
	err   error
	calls int
}

func (p *renewProvider) Watch(context.Context, model.MailConnection) (string, time.Time, error) {
	p.calls++
	return "latest", time.Unix(1000, 0), p.err
}

func TestRenewWatch(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	active := func() *model.MailConnection {
		return &model.MailConnection{ID: uuid.New(), Provider: "gmail", BudgetIDs: []uuid.UUID{uuid.New()}, Cursor: "stored"}
	}
	run := func(conns *renewConns, p *renewProvider, schedules *fakeSchedules) (WatchRenewal, error) {
		return NewMailboxService(MailboxDeps{
			Connections: conns, Providers: oneProvider{p}, Schedules: schedules,
		}).RenewWatch(ctx, uuid.New())
	}

	t.Run("extends the watch and keeps the sync position", func(t *testing.T) {
		conns := &renewConns{conn: active()}
		res, err := run(conns, &renewProvider{}, &fakeSchedules{})
		if err != nil || !res.Renewed || conns.savedCur != "stored" || !conns.savedUntil.Equal(time.Unix(1000, 0)) {
			t.Fatalf("%+v, %v, saved %q", res, err, conns.savedCur)
		}
	})

	t.Run("a mailbox with no position takes the new one", func(t *testing.T) {
		conn := active()
		conn.Cursor = ""
		conns := &renewConns{conn: conn}
		if _, err := run(conns, &renewProvider{}, &fakeSchedules{}); err != nil || conns.savedCur != "latest" {
			t.Fatalf("%v, saved %q", err, conns.savedCur)
		}
	})

	t.Run("skips paused and needs-reconnect mailboxes", func(t *testing.T) {
		paused := active()
		paused.PausedAt = &now
		broken := active()
		broken.Status = model.MailConnectionNeedsReconnect
		for _, conn := range []*model.MailConnection{paused, broken} {
			p := &renewProvider{}
			res, err := run(&renewConns{conn: conn}, p, &fakeSchedules{})
			if err != nil || res.Renewed || res.Skipped == "" || p.calls != 0 {
				t.Fatalf("%+v, %v, calls %d", res, err, p.calls)
			}
		}
	})

	t.Run("a deleted mailbox turns its schedule off", func(t *testing.T) {
		schedules := &fakeSchedules{}
		res, err := run(&renewConns{err: errs.New(errs.CodeConnectionNotFound, "gone")}, &renewProvider{}, schedules)
		if err != nil || res.Skipped == "" || len(schedules.disabled) != 1 {
			t.Fatalf("%+v, %v, %v", res, err, schedules.disabled)
		}
	})

	t.Run("refused credentials ask for a reconnect instead of retrying", func(t *testing.T) {
		conns := &renewConns{conn: active()}
		res, err := run(conns, &renewProvider{err: errs.New(errs.CodeMailAuthFailed, "revoked")}, &fakeSchedules{})
		if err != nil || res.Skipped == "" || conns.status != model.MailConnectionNeedsReconnect {
			t.Fatalf("%+v, %v, status %q", res, err, conns.status)
		}
	})

	t.Run("a temporary failure retries", func(t *testing.T) {
		if _, err := run(&renewConns{conn: active()}, &renewProvider{err: errors.New("503")}, &fakeSchedules{}); err == nil {
			t.Fatal("want an error")
		}
	})
}
