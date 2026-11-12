package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
)

// MailProviders picks the adapter for a connection's provider. The factory's
// registry (factory.MailProviderRegistry) satisfies it.
type MailProviders interface {
	For(provider string) (ports.MailProvider, error)
}

type MailboxDeps struct {
	Queue       Enqueuer
	Connections repository.MailConnectionRepository
	Budgets     repository.BudgetRepository
	Providers   MailProviders
	Ingest      IngestService
	// Schedules renews each mailbox's watch. Nil when the job queue can't
	// keep schedules (no queue, or the memory driver).
	Schedules job.ScheduleStore
}

// watchRenewalCron renews every watch daily, as Gmail recommends. A watch lasts
// about 7 days, so a few failed runs in a row still don't lose it.
const (
	watchRenewalCron     = "0 3 * * *"
	watchRenewalTimezone = "UTC"
)

// WatchRenewal is what one RenewWatch did; it becomes the job's result.
type WatchRenewal struct {
	Renewed bool   `json:"renewed"`
	Skipped string `json:"skipped,omitempty"`
}

// Counts is what queuing a batch of messages did.
type Counts struct {
	Queued     int `json:"queued"`     // new message.received jobs
	Duplicates int `json:"duplicates"` // already queued earlier
	Dropped    int `json:"dropped"`    // messages that can never be queued (e.g. empty body)
}

// SyncResult is what one Sync did; it becomes the job's result.
type SyncResult struct {
	// Skipped says why nothing was fetched; empty when the sync ran.
	Skipped string `json:"skipped,omitempty"`
	Fetched int    `json:"fetched"`
	Counts
	Cursor string `json:"cursor,omitempty"`
}

const (
	// backfillDays is how far back a backfill looks.
	backfillDays = 30
	// backfillMaxEmails bounds one backfill, which is every email in the window.
	backfillMaxEmails = 2000
)

// BackfillResult is what one Backfill did.
type BackfillResult struct {
	Counts
}

type MailboxService interface {
	// Changed queues a model.JobTypeMailboxChanged job for one provider
	// notification. The same notification delivered twice (Pub/Sub is at
	// least once) maps to the same job: the idempotency key is
	// provider:account:cursor.
	Changed(ctx context.Context, n model.MailboxChanged) (job.EnqueueResult, error)

	// Connect attaches the mailbox behind a consent code to budgetID and starts
	// watching it from now. Connecting it again replaces its credentials and
	// restarts the watch, so mail from before this call isn't fetched.
	Connect(ctx context.Context, budgetID uuid.UUID, provider, code string) (*model.MailConnection, error)

	// BackfillCandidates lists the recent emails in one of budgetID's mailboxes,
	// without bodies, for the user to choose from. Nothing is queued.
	BackfillCandidates(ctx context.Context, budgetID, connectionID uuid.UUID) ([]ports.InboundMessage, error)

	// QueueBackfill queues the job that imports the emails named by externalIDs,
	// at most as many as BackfillCandidates lists. The same choice queued twice
	// maps to the same job. Returns CodeConnectionNotFound for another budget's
	// mailbox.
	QueueBackfill(
		ctx context.Context,
		budgetID, connectionID uuid.UUID,
		externalIDs []string,
	) (job.EnqueueResult, error)

	// Backfill queues the emails named by externalIDs for budgetID. An ID the
	// mailbox doesn't have is ignored. Re-importing an email is a duplicate, not
	// a second job. opts apply to each queued message.
	Backfill(
		ctx context.Context,
		budgetID, connectionID uuid.UUID,
		externalIDs []string,
		opts ...job.EnqueueOption,
	) (BackfillResult, error)

	// Disconnect removes a mailbox from budgetID. A mailbox left with no
	// budgets is deleted with its credentials and stops being watched.
	Disconnect(ctx context.Context, budgetID, connectionID uuid.UUID) error

	// SetPaused pauses or resumes a mailbox, for every budget it feeds. Pausing
	// stops the watch. Resuming starts a new one from now, so mail from the
	// pause isn't fetched. Asking for the current state changes nothing. A
	// mailbox whose access has ended resumes as needing a reconnect.
	SetPaused(ctx context.Context, budgetID, connectionID uuid.UUID, paused bool) (*model.MailConnection, error)

	// ListConnections lists the mailboxes feeding budgetID.
	ListConnections(ctx context.Context, budgetID uuid.UUID) ([]model.MailConnection, error)

	// Test queues the job a provider push would, for a mailbox connected to
	// budgetID. Each call gets its own cursor, so repeats aren't deduped; the
	// sync itself starts from the connection's cursor either way. Returns
	// CodeConnectionNotFound for a mailbox the budget isn't connected to.
	Test(ctx context.Context, budgetID uuid.UUID, provider, account string) (job.EnqueueResult, error)

	// Sync fetches what arrived in the notified mailbox since its stored
	// cursor, queues each message for every live budget of the connection,
	// then saves the new cursor.
	//
	// Problems a retry can't fix (no connection, disabled, credentials
	// refused, cursor expired) are recorded on the connection and returned as
	// a Skipped result with a nil error, so the job completes instead of
	// retrying. A returned error means "try again": the cursor isn't moved,
	// so the retry fetches the same messages and the idempotency keys drop
	// the ones already queued.
	//
	// opts apply to each queued message.
	Sync(ctx context.Context, n model.MailboxChanged, opts ...job.EnqueueOption) (SyncResult, error)

	// RenewWatch extends a mailbox's push notifications, leaving its sync
	// position where it is. A mailbox that's gone, paused or needs
	// reconnecting is skipped with a nil error, so the job doesn't retry.
	RenewWatch(ctx context.Context, connectionID uuid.UUID) (WatchRenewal, error)
}

type mailboxService struct {
	deps MailboxDeps
}

func NewMailboxService(deps MailboxDeps) MailboxService {
	return &mailboxService{deps: deps}
}

func (s *mailboxService) Changed(ctx context.Context, n model.MailboxChanged) (job.EnqueueResult, error) {
	n.Provider = strings.ToLower(strings.TrimSpace(n.Provider))
	// Gmail reports the address as typed at signup; compare case-insensitively.
	n.Account = strings.ToLower(strings.TrimSpace(n.Account))
	if n.Provider == "" || n.Account == "" || n.Cursor == "" {
		return job.EnqueueResult{}, errs.New(errs.CodeIngestInvalidMessage,
			"mailbox notification needs provider, account and cursor")
	}

	payload, err := json.Marshal(n)
	if err != nil {
		return job.EnqueueResult{}, errs.Wrap(errs.CodeInternalError, "encode notification", err)
	}
	return s.deps.Queue.Enqueue(ctx, model.JobTypeMailboxChanged, payload,
		job.WithQueue(model.QueueIngest),
		job.WithIdempotencyKey(n.Provider+":"+n.Account+":"+n.Cursor))
}

func (s *mailboxService) Connect(
	ctx context.Context,
	budgetID uuid.UUID,
	providerName, code string,
) (*model.MailConnection, error) {
	provider, err := s.deps.Providers.For(providerName)
	if err != nil {
		return nil, err
	}
	mailbox, err := provider.Connect(ctx, code)
	if err != nil {
		return nil, err
	}
	conn, err := s.deps.Connections.Upsert(ctx, providerName, mailbox.Account, mailbox.Credentials, budgetID)
	if err != nil {
		return nil, err
	}

	cursor, expiresAt, err := provider.Watch(ctx, *conn)
	if err != nil {
		s.recordFailure(ctx, conn.ID, err)
		return nil, err
	}
	if err := s.deps.Connections.SaveWatch(ctx, conn.ID, cursor, expiresAt); err != nil {
		return nil, err
	}
	conn.Cursor, conn.WatchExpiry = cursor, &expiresAt
	if err := s.ensureWatchSchedule(ctx, conn.ID); err != nil {
		return nil, err
	}
	return conn, nil
}

// ensureWatchSchedule creates the mailbox's renewal schedule, or turns it back
// on. A mailbox without one loses its pushes when the watch runs out, so a
// failure here fails the request.
func (s *mailboxService) ensureWatchSchedule(ctx context.Context, connectionID uuid.UUID) error {
	if s.deps.Schedules == nil {
		return nil
	}
	payload, err := json.Marshal(model.MailboxWatch{ConnectionID: connectionID})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "encode watch schedule", err)
	}
	next, err := jobrunner.NextCronRun(watchRenewalCron, watchRenewalTimezone, time.Now())
	if err != nil {
		return err
	}
	return s.deps.Schedules.EnsureSchedule(ctx, job.JobSchedule{
		Name:      model.MailboxWatchScheduleName(connectionID),
		Type:      model.JobTypeMailboxStartWatch,
		Queue:     model.QueueIngest,
		Payload:   payload,
		Cron:      watchRenewalCron,
		Timezone:  watchRenewalTimezone,
		NextRunAt: next,
	})
}

// disableWatchSchedule is best effort: RenewWatch skips a paused or deleted
// mailbox anyway.
func (s *mailboxService) disableWatchSchedule(ctx context.Context, connectionID uuid.UUID) {
	if s.deps.Schedules == nil {
		return
	}
	if err := s.deps.Schedules.DisableSchedule(ctx, model.MailboxWatchScheduleName(connectionID)); err != nil {
		logger.Logger(ctx).Warn("disable watch schedule failed", "connection_id", connectionID, "err", err)
	}
}

// RenewWatch implements [MailboxService].
func (s *mailboxService) RenewWatch(ctx context.Context, connectionID uuid.UUID) (WatchRenewal, error) {
	conn, err := s.deps.Connections.GetByID(ctx, connectionID)
	if errs.HasCode(err, errs.CodeConnectionNotFound) {
		s.disableWatchSchedule(ctx, connectionID)
		return WatchRenewal{Skipped: "connection is gone"}, nil
	}
	if err != nil {
		return WatchRenewal{}, err
	}
	if skip := unusable(conn); skip != "" {
		return WatchRenewal{Skipped: skip}, nil
	}

	provider, err := s.deps.Providers.For(conn.Provider)
	if err != nil {
		return WatchRenewal{}, err
	}
	cursor, expiresAt, err := provider.Watch(ctx, *conn)
	if err != nil {
		// A temporary failure isn't recorded: the old watch still runs.
		status, skip := failure(err)
		if skip == "" {
			return WatchRenewal{}, err
		}
		if err := s.deps.Connections.SaveFailure(ctx, conn.ID, status, err.Error()); err != nil {
			return WatchRenewal{}, err
		}
		return WatchRenewal{Skipped: skip}, nil
	}
	// Keep the stored cursor: Watch returns the mailbox's latest position, and
	// moving to it would skip anything not synced yet.
	if conn.Cursor != "" {
		cursor = conn.Cursor
	}
	if err := s.deps.Connections.SaveWatch(ctx, conn.ID, cursor, expiresAt); err != nil {
		return WatchRenewal{}, err
	}
	return WatchRenewal{Renewed: true}, nil
}

func (s *mailboxService) ListConnections(ctx context.Context, budgetID uuid.UUID) ([]model.MailConnection, error) {
	return s.deps.Connections.ListByBudget(ctx, budgetID)
}

func (s *mailboxService) Disconnect(ctx context.Context, budgetID, connectionID uuid.UUID) error {
	conn, err := s.connectionFor(ctx, budgetID, connectionID)
	if err != nil {
		return err
	}
	detached, err := s.deps.Connections.DetachBudget(ctx, conn.ID, budgetID)
	if err != nil {
		return err
	}
	if len(detached.BudgetIDs) == 0 {
		s.stopWatch(ctx, conn)
		s.disableWatchSchedule(ctx, conn.ID)
	}
	return nil
}

func (s *mailboxService) SetPaused(
	ctx context.Context,
	budgetID, connectionID uuid.UUID,
	paused bool,
) (*model.MailConnection, error) {
	conn, err := s.connectionFor(ctx, budgetID, connectionID)
	if err != nil {
		return nil, err
	}
	if paused == (conn.PausedAt != nil) {
		return conn, nil
	}
	if paused {
		updated, err := s.deps.Connections.SetPaused(ctx, conn.ID, true)
		if err != nil {
			return nil, err
		}
		s.stopWatch(ctx, conn)
		s.disableWatchSchedule(ctx, conn.ID)
		return updated, nil
	}

	provider, err := s.deps.Providers.For(conn.Provider)
	if err != nil {
		return nil, err
	}
	cursor, expiresAt, err := provider.Watch(ctx, *conn)
	if err != nil {
		// Only a failure retrying can't fix resumes the mailbox, in that state.
		status, skip := failure(err)
		if skip == "" {
			return nil, err
		}
		if err := s.deps.Connections.SaveFailure(ctx, conn.ID, status, err.Error()); err != nil {
			return nil, err
		}
	} else if err := s.deps.Connections.SaveWatch(ctx, conn.ID, cursor, expiresAt); err != nil {
		return nil, err
	}
	// Before unpausing: once unpaused, a retry returns early and would never
	// get here.
	if err := s.ensureWatchSchedule(ctx, conn.ID); err != nil {
		return nil, err
	}
	return s.deps.Connections.SetPaused(ctx, conn.ID, false)
}

// stopWatch ends a mailbox's push notifications. It's best effort: Sync skips a
// paused or deleted mailbox anyway, and the watch runs out by itself. Failing
// would strand a user whose Google access has already ended.
func (s *mailboxService) stopWatch(ctx context.Context, conn *model.MailConnection) {
	provider, err := s.deps.Providers.For(conn.Provider)
	if err == nil {
		err = provider.StopWatch(ctx, *conn)
	}
	if err != nil {
		logger.Logger(ctx).Warn("stop watch failed", "connection_id", conn.ID, "err", err)
	}
}

func (s *mailboxService) Test(
	ctx context.Context,
	budgetID uuid.UUID,
	provider, account string,
) (job.EnqueueResult, error) {
	conn, err := s.deps.Connections.GetByAccount(ctx, provider, account)
	if err != nil {
		return job.EnqueueResult{}, err
	}
	if err := requireBudget(conn, budgetID); err != nil {
		return job.EnqueueResult{}, err
	}
	return s.Changed(ctx, model.MailboxChanged{
		Provider: conn.Provider,
		Account:  conn.Account,
		Cursor:   "test-" + uuid.NewString(),
	})
}

// connectionFor returns a connection budgetID may use. Another budget's mailbox
// reads as missing.
func (s *mailboxService) connectionFor(ctx context.Context, budgetID, id uuid.UUID) (*model.MailConnection, error) {
	conn, err := s.deps.Connections.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := requireBudget(conn, budgetID); err != nil {
		return nil, err
	}
	return conn, nil
}

// requireBudget fails unless budgetID is one of conn's budgets. Another
// budget's mailbox reads as missing.
func requireBudget(conn *model.MailConnection, budgetID uuid.UUID) error {
	if !slices.Contains(conn.BudgetIDs, budgetID) {
		return errs.New(errs.CodeConnectionNotFound, "mailbox is not connected to this budget")
	}
	return nil
}

func (s *mailboxService) BackfillCandidates(
	ctx context.Context,
	budgetID, connectionID uuid.UUID,
) ([]ports.InboundMessage, error) {
	conn, err := s.connectionFor(ctx, budgetID, connectionID)
	if err != nil {
		return nil, err
	}
	provider, err := s.deps.Providers.For(conn.Provider)
	if err != nil {
		return nil, err
	}
	return provider.Search(ctx, *conn, ports.Query{Days: backfillDays, MaxResults: backfillMaxEmails})
}

func (s *mailboxService) QueueBackfill(
	ctx context.Context,
	budgetID, connectionID uuid.UUID,
	externalIDs []string,
) (job.EnqueueResult, error) {
	if len(externalIDs) > backfillMaxEmails {
		return job.EnqueueResult{}, errs.New(errs.CodeIngestInvalidMessage,
			"choose at most %d emails", backfillMaxEmails)
	}
	conn, err := s.connectionFor(ctx, budgetID, connectionID)
	if err != nil {
		return job.EnqueueResult{}, err
	}

	ids := slices.Compact(slices.Sorted(slices.Values(externalIDs))) // the same choice, however it's listed
	payload, err := json.Marshal(model.MailboxBackfill{
		ConnectionID: conn.ID,
		Account:      conn.Account,
		ExternalIDs:  ids,
	})
	if err != nil {
		return job.EnqueueResult{}, errs.Wrap(errs.CodeInternalError, "encode backfill", err)
	}
	choice := sha256.Sum256([]byte(strings.Join(ids, "\x00")))
	return s.deps.Queue.Enqueue(ctx, model.JobTypeMailboxBackfill, payload,
		job.WithQueue(model.QueueIngest),
		job.WithBudgetID(&budgetID),
		job.WithIdempotencyKey(budgetID.String()+":"+conn.ID.String()+":"+hex.EncodeToString(choice[:])))
}

func (s *mailboxService) Backfill(
	ctx context.Context,
	budgetID, connectionID uuid.UUID,
	externalIDs []string,
	opts ...job.EnqueueOption,
) (BackfillResult, error) {
	conn, err := s.connectionFor(ctx, budgetID, connectionID)
	if err != nil {
		return BackfillResult{}, err
	}
	provider, err := s.deps.Providers.For(conn.Provider)
	if err != nil {
		return BackfillResult{}, err
	}
	jobrunner.ReportProgress(ctx, job.Progress{Message: "Reading the emails", Total: len(externalIDs)})
	messages, err := provider.Fetch(ctx, *conn, externalIDs)
	if err != nil {
		return BackfillResult{}, err
	}

	counts, err := s.queueAll(ctx, []uuid.UUID{budgetID}, messages, opts)
	return BackfillResult{Counts: counts}, err
}

// queueAll accepts each message for budgets, reporting progress as it goes.
// A message that can never be queued is dropped, so it can't block the rest;
// any other failure stops the batch for a retry.
func (s *mailboxService) queueAll(
	ctx context.Context,
	budgets []uuid.UUID,
	messages []ports.InboundMessage,
	opts []job.EnqueueOption,
) (Counts, error) {
	var counts Counts
	for i, msg := range messages {
		jobrunner.ReportProgress(ctx, job.Progress{Message: "Queuing emails", Current: i + 1, Total: len(messages)})
		accepted, err := s.deps.Ingest.Accept(ctx, budgets, msg, opts...)
		if errs.HasCode(err, errs.CodeIngestInvalidMessage) {
			logger.Logger(ctx).Warn("dropping message that can't be queued", "external_id", msg.ExternalID, "err", err)
			counts.Dropped++
			continue
		}
		if err != nil {
			return counts, err
		}
		for _, q := range accepted.Jobs {
			if q.Duplicate {
				counts.Duplicates++
			} else {
				counts.Queued++
			}
		}
	}
	return counts, nil
}

func (s *mailboxService) Sync(
	ctx context.Context,
	n model.MailboxChanged,
	opts ...job.EnqueueOption,
) (SyncResult, error) {
	log := logger.Logger(ctx)
	jobrunner.ReportProgress(ctx, job.Progress{Message: "Reading the mailbox"})

	conn, err := s.deps.Connections.GetByAccount(ctx, n.Provider, n.Account)
	if errs.HasCode(err, errs.CodeConnectionNotFound) {
		// e.g. a mailbox go-gmail still watches but ingest doesn't know yet
		return SyncResult{Skipped: "no connection for this mailbox"}, nil
	}
	if err != nil {
		return SyncResult{}, err
	}
	if skip := unusable(conn); skip != "" {
		return SyncResult{Skipped: skip}, nil
	}
	budgets, err := s.liveBudgets(ctx, conn.BudgetIDs)
	if err != nil {
		return SyncResult{}, err
	}
	if len(budgets) == 0 {
		return SyncResult{Skipped: "none of the connection's budgets exist"}, nil
	}
	provider, err := s.deps.Providers.For(conn.Provider)
	if err != nil {
		return SyncResult{}, err // config problem: retried, then dead
	}

	messages, cursor, err := provider.FetchSince(ctx, *conn)
	if err != nil {
		if skip := s.recordFailure(ctx, conn.ID, err); skip != "" {
			log.Warn("mailbox sync skipped", "connection_id", conn.ID, "reason", skip, "err", err)
			return SyncResult{Skipped: skip}, nil
		}
		return SyncResult{}, err
	}

	counts, err := s.queueAll(ctx, budgets, messages, opts)
	if err != nil {
		return SyncResult{}, err // cursor not saved: the retry refetches
	}
	// Only now, with every message queued, move the cursor forward.
	if err := s.deps.Connections.SaveSync(ctx, conn.ID, cursor); err != nil {
		return SyncResult{}, err
	}
	res := SyncResult{Fetched: len(messages), Counts: counts, Cursor: cursor}
	log.Info("mailbox synced", "connection_id", conn.ID, "fetched", res.Fetched,
		"queued", res.Queued, "duplicates", res.Duplicates, "dropped", res.Dropped)
	return res, nil
}

// unusable says why a connection can't be synced or watched right now; ""
// when it can.
func unusable(conn *model.MailConnection) string {
	switch {
	case len(conn.BudgetIDs) == 0:
		return "connection has no budgets"
	case conn.PausedAt != nil:
		return "connection is paused"
	case conn.Status == model.MailConnectionNeedsReconnect:
		return "connection needs reconnecting"
	}
	return ""
}

// failure is what a provider error means for a connection: the status to
// show, and why retrying won't help ("" when it might).
func failure(err error) (status model.MailConnectionStatus, permanent string) {
	switch {
	case errs.HasCode(err, errs.CodeMailAuthFailed):
		return model.MailConnectionNeedsReconnect, "credentials refused; user must reconnect"
	case errs.HasCode(err, errs.CodeMailCursorExpired):
		return model.MailConnectionError, "history cursor expired; mailbox needs a new watch and a backfill"
	case errs.HasCode(err, errs.CodeMailInvalidConnection):
		return model.MailConnectionError, "connection row is unusable (credentials or cursor)"
	}
	return model.MailConnectionError, ""
}

// recordFailure writes err to the connection, best effort, and returns
// failure's reason retrying won't help.
func (s *mailboxService) recordFailure(ctx context.Context, connectionID uuid.UUID, err error) string {
	status, permanent := failure(err)
	if saveErr := s.deps.Connections.SaveFailure(ctx, connectionID, status, err.Error()); saveErr != nil {
		logger.Logger(ctx).Error("record connection failure", "connection_id", connectionID, "err", saveErr)
	}
	return permanent
}

// liveBudgets drops budgets that were deleted since they were attached
// (budget_ids has no foreign key).
func (s *mailboxService) liveBudgets(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	live := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		_, err := s.deps.Budgets.GetById(ctx, id)
		if errors.Is(err, repository.ErrNotFound) {
			continue // deleted (budgets are soft-deleted)
		}
		if err != nil {
			return nil, err
		}
		live = append(live, id)
	}
	return live, nil
}
