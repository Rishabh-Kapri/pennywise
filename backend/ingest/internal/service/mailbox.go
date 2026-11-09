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
// registry (factory.MailProviders) satisfies it.
type MailProviders interface {
	For(provider string) (ports.MailProvider, error)
}

// MailboxDeps is what Sync needs. Changed only needs the broker.
type MailboxDeps struct {
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
const watchRenewalCron = "0 3 * * *"

// WatchRenewal is what one RenewWatch did; it becomes the job's result.
type WatchRenewal struct {
	Renewed bool   `json:"renewed"`
	Skipped string `json:"skipped,omitempty"`
}

// SyncResult is what one Sync did; it becomes the job's result.
type SyncResult struct {
	// Skipped says why nothing was fetched; empty when the sync ran.
	Skipped    string `json:"skipped,omitempty"`
	Fetched    int    `json:"fetched"`
	Queued     int    `json:"queued"`     // new message.received jobs
	Duplicates int    `json:"duplicates"` // already queued earlier
	Dropped    int    `json:"dropped"`    // messages that can never be queued (e.g. empty body)
	Cursor     string `json:"cursor,omitempty"`
}

const (
	// backfillDays is how far back a backfill looks.
	backfillDays = 30
	// backfillMaxEmails bounds one backfill, which is every email in the window.
	backfillMaxEmails = 2000
)

// BackfillResult is what one Backfill did.
type BackfillResult struct {
	Queued     int `json:"queued"`
	Duplicates int `json:"duplicates"` // imported earlier
	Dropped    int `json:"dropped"`    // messages that can never be queued (e.g. empty body)
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

	// QueueBackfill queues the job that imports the emails named by externalIDs.
	// The same choice queued twice maps to the same job. Returns
	// CodeConnectionNotFound for another budget's mailbox.
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
	broker job.Broker // nil when no job queue is configured
	deps   MailboxDeps
}

func NewMailboxService(broker job.Broker, deps MailboxDeps) MailboxService {
	return &mailboxService{broker: broker, deps: deps}
}

func (s *mailboxService) Changed(ctx context.Context, n model.MailboxChanged) (job.EnqueueResult, error) {
	n.Provider = strings.ToLower(strings.TrimSpace(n.Provider))
	// Gmail reports the address as typed at signup; compare case-insensitively.
	n.Account = strings.ToLower(strings.TrimSpace(n.Account))
	if n.Provider == "" || n.Account == "" || n.Cursor == "" {
		return job.EnqueueResult{}, errs.New(errs.CodeIngestInvalidMessage,
			"mailbox notification needs provider, account and cursor")
	}
	if s.broker == nil {
		return job.EnqueueResult{}, errs.New(errs.CodeIngestQueueUnavailable, "no job queue configured")
	}

	payload, err := json.Marshal(n)
	if err != nil {
		return job.EnqueueResult{}, errs.Wrap(errs.CodeInternalError, "encode notification", err)
	}
	return s.broker.Enqueue(ctx, model.JobTypeMailboxChanged, payload,
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
		if saveErr := s.deps.Connections.SaveFailure(
			ctx, conn.ID, model.MailConnectionError, err.Error(),
		); saveErr != nil {
			logger.Logger(ctx).Error("record failed watch", "connection_id", conn.ID, "err", saveErr)
		}
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

func watchScheduleName(connectionID uuid.UUID) string {
	return "mailbox.watch:" + connectionID.String()
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
	next, err := jobrunner.NextCronRun(watchRenewalCron, "UTC", time.Now())
	if err != nil {
		return err
	}
	return s.deps.Schedules.EnsureSchedule(ctx, job.JobSchedule{
		Name:      watchScheduleName(connectionID),
		Type:      model.JobTypeMailboxStartWatch,
		Queue:     model.QueueIngest,
		Payload:   payload,
		Cron:      watchRenewalCron,
		Timezone:  "UTC",
		NextRunAt: next,
	})
}

// disableWatchSchedule is best effort: RenewWatch skips a paused or deleted
// mailbox anyway.
func (s *mailboxService) disableWatchSchedule(ctx context.Context, connectionID uuid.UUID) {
	if s.deps.Schedules == nil {
		return
	}
	if err := s.deps.Schedules.DisableSchedule(ctx, watchScheduleName(connectionID)); err != nil {
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
	switch {
	case conn.PausedAt != nil:
		return WatchRenewal{Skipped: "connection is paused"}, nil
	case len(conn.BudgetIDs) == 0:
		return WatchRenewal{Skipped: "connection has no budgets"}, nil
	case conn.Status == model.MailConnectionNeedsReconnect:
		return WatchRenewal{Skipped: "connection needs reconnecting"}, nil
	}

	provider, err := s.deps.Providers.For(conn.Provider)
	if err != nil {
		return WatchRenewal{}, err
	}
	cursor, expiresAt, err := provider.Watch(ctx, *conn)
	if errs.HasCode(err, errs.CodeMailAuthFailed) {
		if saveErr := s.deps.Connections.SaveFailure(
			ctx, conn.ID, model.MailConnectionNeedsReconnect, err.Error(),
		); saveErr != nil {
			return WatchRenewal{}, saveErr
		}
		return WatchRenewal{Skipped: "credentials were refused"}, nil
	}
	if err != nil {
		return WatchRenewal{}, err
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
	switch {
	case errs.HasCode(err, errs.CodeMailAuthFailed):
		if err := s.deps.Connections.SaveFailure(
			ctx, conn.ID, model.MailConnectionNeedsReconnect, err.Error(),
		); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		if err := s.deps.Connections.SaveWatch(ctx, conn.ID, cursor, expiresAt); err != nil {
			return nil, err
		}
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
	if !slices.Contains(conn.BudgetIDs, budgetID) {
		return job.EnqueueResult{}, errs.New(errs.CodeConnectionNotFound, "mailbox is not connected to this budget")
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
	if !slices.Contains(conn.BudgetIDs, budgetID) {
		return nil, errs.New(errs.CodeConnectionNotFound, "mailbox is not connected to this budget")
	}
	return conn, nil
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
	conn, err := s.connectionFor(ctx, budgetID, connectionID)
	if err != nil {
		return job.EnqueueResult{}, err
	}
	if s.broker == nil {
		return job.EnqueueResult{}, errs.New(errs.CodeIngestQueueUnavailable, "no job queue configured")
	}

	ids := slices.Sorted(slices.Values(externalIDs))
	payload, err := json.Marshal(model.MailboxBackfill{
		ConnectionID: conn.ID,
		Account:      conn.Account,
		ExternalIDs:  ids,
	})
	if err != nil {
		return job.EnqueueResult{}, errs.Wrap(errs.CodeInternalError, "encode backfill", err)
	}
	choice := sha256.Sum256([]byte(strings.Join(ids, "\x00")))
	return s.broker.Enqueue(ctx, model.JobTypeMailboxBackfill, payload,
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
	messages, err := provider.Fetch(ctx, *conn, externalIDs)
	if err != nil {
		return BackfillResult{}, err
	}

	var res BackfillResult
	for _, msg := range messages {
		msg.BudgetID = []uuid.UUID{budgetID}
		queued, err := s.deps.Ingest.Accept(ctx, msg, opts...)
		if errs.HasCode(err, errs.CodeIngestInvalidMessage) {
			res.Dropped++
			continue
		}
		if err != nil {
			return res, err
		}
		for _, q := range queued {
			if q.Duplicate {
				res.Duplicates++
			} else {
				res.Queued++
			}
		}
	}
	return res, nil
}

// fetched is what Sync and Preview share: the connection, its live
// budgets, and what the provider returned.
type fetched struct {
	conn     *model.MailConnection
	budgets  []uuid.UUID
	messages []ports.InboundMessage
	cursor   string
}

// fetch looks the connection up and fetches from its cursor. A non-empty
// skip means there is nothing to do (and no error to retry).
func (s *mailboxService) fetch(ctx context.Context, n model.MailboxChanged) (*fetched, string, error) {
	conn, err := s.deps.Connections.GetByAccount(ctx, n.Provider, n.Account)
	if errs.HasCode(err, errs.CodeConnectionNotFound) {
		// e.g. a mailbox go-gmail still watches but ingest doesn't know yet
		return nil, "no connection for this mailbox", nil
	}
	if err != nil {
		return nil, "", err
	}
	if len(conn.BudgetIDs) == 0 {
		return nil, "connection is disabled", nil
	}
	if conn.PausedAt != nil {
		return nil, "connection is paused", nil
	}
	if conn.Status == model.MailConnectionNeedsReconnect {
		return nil, "connection needs reconnecting", nil
	}

	budgets, err := s.liveBudgets(ctx, conn.BudgetIDs)
	if err != nil {
		return nil, "", err
	}
	if len(budgets) == 0 {
		return nil, "none of the connection's budgets exist", nil
	}

	provider, err := s.deps.Providers.For(conn.Provider)
	if err != nil {
		return nil, "", err // config problem: retried, then dead
	}

	messages, cursor, err := provider.FetchSince(ctx, *conn)
	if err != nil {
		skip, err := s.fetchFailed(ctx, conn, err)
		return nil, skip, err
	}
	return &fetched{conn: conn, budgets: budgets, messages: messages, cursor: cursor}, "", nil
}

func (s *mailboxService) Sync(
	ctx context.Context,
	n model.MailboxChanged,
	opts ...job.EnqueueOption,
) (SyncResult, error) {
	log := logger.Logger(ctx)
	f, skip, err := s.fetch(ctx, n)
	if err != nil || skip != "" {
		return SyncResult{Skipped: skip}, err
	}
	conn, budgets, messages, cursor := f.conn, f.budgets, f.messages, f.cursor

	res := SyncResult{Fetched: len(messages), Cursor: cursor}
	for _, msg := range messages {
		msg.BudgetID = budgets // only budgets that still exist
		queued, err := s.deps.Ingest.Accept(ctx, msg, opts...)
		if errs.HasCode(err, errs.CodeIngestInvalidMessage) {
			// It will be just as invalid next time; don't block the mailbox on it.
			log.Warn("dropping message that can't be queued",
				"connection_id", conn.ID, "external_id", msg.ExternalID, "err", err)
			res.Dropped++
			continue
		}
		if err != nil {
			return SyncResult{}, err // cursor not saved: the retry refetches
		}
		for _, q := range queued {
			if q.Duplicate {
				res.Duplicates++
			} else {
				res.Queued++
			}
		}
	}

	// Only now, with every message queued, move the cursor forward.
	if err := s.deps.Connections.SaveSync(ctx, conn.ID, cursor); err != nil {
		return SyncResult{}, err
	}
	log.Info("mailbox synced", "connection_id", conn.ID, "fetched", res.Fetched,
		"queued", res.Queued, "duplicates", res.Duplicates, "dropped", res.Dropped)
	return res, nil
}

// fetchFailed classifies a failed fetch and writes it to the connection. Failures only the user (or a new watch) can fix return
// a skip reason and no error; anything else is returned for a retry.
func (s *mailboxService) fetchFailed(ctx context.Context, conn *model.MailConnection, err error) (string, error) {
	status, skip := model.MailConnectionError, ""
	switch {
	case errs.HasCode(err, errs.CodeMailAuthFailed):
		status, skip = model.MailConnectionNeedsReconnect, "credentials refused; user must reconnect"
	case errs.HasCode(err, errs.CodeMailCursorExpired):
		skip = "history cursor expired; mailbox needs a new watch and a backfill"
	case errs.HasCode(err, errs.CodeMailInvalidConnection):
		skip = "connection row is unusable (credentials or cursor)"
	}
	if saveErr := s.deps.Connections.SaveFailure(ctx, conn.ID, status, err.Error()); saveErr != nil {
		logger.Logger(ctx).Error("record sync failure", "connection_id", conn.ID, "err", saveErr)
	}
	if skip != "" {
		logger.Logger(ctx).Warn("mailbox sync skipped", "connection_id", conn.ID, "reason", skip, "err", err)
		return skip, nil
	}
	return "", err
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
