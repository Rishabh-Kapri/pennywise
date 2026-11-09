package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
)

// mail_connections: connected mailboxes for ingest (migration 00014).

const mailConnectionColumns = `id, provider, account, budget_ids, credentials, sync_cursor,
	watch_expires_at, status, last_sync_at, last_error, paused_at, created_at, updated_at`

type mailConnectionRepo struct {
	BaseRepository
}

func NewMailConnectionRepository(pool *pgxpool.Pool) repository.MailConnectionRepository {
	return &mailConnectionRepo{BaseRepository: NewBaseRepository(pool)}
}

// Upsert implements [repository.MailConnectionRepository]. The ON CONFLICT target
// repeats the partial index's WHERE, or Postgres finds no matching
// constraint.
func (r *mailConnectionRepo) Upsert(
	ctx context.Context,
	provider, account string,
	credentials []byte,
	budgetID uuid.UUID,
) (*model.MailConnection, error) {
	provider, account = normalizeMail(provider), normalizeMail(account)
	c, err := scanMailConnection(r.Executor(ctx).QueryRow(
		ctx, `
		INSERT INTO mail_connections (provider, account, credentials, budget_ids)
		VALUES ($1, $2, $3, ARRAY[$4::uuid])
		ON CONFLICT (provider, account) WHERE deleted_at IS NULL DO UPDATE SET
			credentials = EXCLUDED.credentials,
			budget_ids = CASE
				WHEN $4::uuid = ANY(mail_connections.budget_ids) THEN mail_connections.budget_ids
				ELSE array_append(mail_connections.budget_ids, $4::uuid)
			END,
			status = 'active',
			last_error = NULL,
			paused_at = NULL,
			updated_at = now()
		RETURNING `+mailConnectionColumns,
		provider, account, credentials, budgetID,
	))
	if err != nil {
		return nil, errs.Wrap(errs.CodeConnectionUpdateFailed, "upsert connection", err,
			errs.WithDetails(map[string]any{"provider": provider, "budget_id": budgetID}))
	}
	return c, nil
}

// GetByID implements [repository.MailConnectionRepository].
func (r *mailConnectionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.MailConnection, error) {
	c, err := scanMailConnection(r.Executor(ctx).QueryRow(ctx,
		`SELECT `+mailConnectionColumns+` FROM mail_connections WHERE id = $1 AND deleted_at IS NULL`, id))
	return mailConnectionResult(c, err, "get connection", errs.WithDetail("connection_id", id))
}

// GetByAccount implements [repository.MailConnectionRepository].
func (r *mailConnectionRepo) GetByAccount(
	ctx context.Context,
	provider, account string,
) (*model.MailConnection, error) {
	provider = normalizeMail(provider)
	c, err := scanMailConnection(r.Executor(ctx).QueryRow(ctx, `
		SELECT `+mailConnectionColumns+` FROM mail_connections
		WHERE provider = $1 AND account = $2 AND deleted_at IS NULL`,
		provider, normalizeMail(account)))
	// The account (an email address) stays out of the error details.
	return mailConnectionResult(c, err, "get connection by account", errs.WithDetail("provider", provider))
}

// ListByBudget implements [repository.MailConnectionRepository].
func (r *mailConnectionRepo) ListByBudget(ctx context.Context, budgetID uuid.UUID) ([]model.MailConnection, error) {
	return r.list(ctx, "list connections by budget", `
		SELECT `+mailConnectionColumns+` FROM mail_connections
		WHERE $1 = ANY(budget_ids) AND deleted_at IS NULL
		ORDER BY created_at`, budgetID)
}

// ListWatchesExpiring implements [repository.MailConnectionRepository]. Connections
// waiting for the user to reconnect are left out, as their credentials can't
// renew a watch, and so are paused ones, which have none to renew.
func (r *mailConnectionRepo) ListWatchesExpiring(
	ctx context.Context,
	before time.Time,
) ([]model.MailConnection, error) {
	return r.list(ctx, "list expiring watches", `
		SELECT `+mailConnectionColumns+` FROM mail_connections
		WHERE deleted_at IS NULL
			AND budget_ids <> '{}'
			AND status <> 'needs_reconnect'
			AND paused_at IS NULL
			AND (watch_expires_at IS NULL OR watch_expires_at < $1)
		ORDER BY watch_expires_at NULLS FIRST`, before)
}

// DetachBudget implements [repository.MailConnectionRepository].
func (r *mailConnectionRepo) DetachBudget(ctx context.Context, id, budgetID uuid.UUID) (*model.MailConnection, error) {
	c, err := scanMailConnection(r.Executor(ctx).QueryRow(ctx, `
		UPDATE mail_connections
		SET budget_ids = array_remove(budget_ids, $2::uuid),
			credentials = CASE
				WHEN cardinality(array_remove(budget_ids, $2::uuid)) = 0 THEN ''::bytea
				ELSE credentials
			END,
			deleted_at = CASE
				WHEN cardinality(array_remove(budget_ids, $2::uuid)) = 0 THEN now()
				ELSE deleted_at
			END,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+mailConnectionColumns, id, budgetID))
	return mailConnectionResult(c, err, "detach budget",
		errs.WithDetails(map[string]any{"connection_id": id, "budget_id": budgetID}))
}

// SetPaused implements [repository.MailConnectionRepository].
func (r *mailConnectionRepo) SetPaused(ctx context.Context, id uuid.UUID, paused bool) (*model.MailConnection, error) {
	c, err := scanMailConnection(r.Executor(ctx).QueryRow(ctx, `
		UPDATE mail_connections
		SET paused_at = CASE WHEN $2 THEN now() END, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+mailConnectionColumns, id, paused))
	return mailConnectionResult(c, err, "set paused", errs.WithDetail("connection_id", id))
}

// SaveWatch implements [repository.MailConnectionRepository].
func (r *mailConnectionRepo) SaveWatch(ctx context.Context, id uuid.UUID, cursor string, expiresAt time.Time) error {
	return r.update(ctx, "save watch", id, `sync_cursor = $2, watch_expires_at = $3`, cursor, expiresAt)
}

// SaveSync implements [repository.MailConnectionRepository].
func (r *mailConnectionRepo) SaveSync(ctx context.Context, id uuid.UUID, cursor string) error {
	return r.update(ctx, "save sync", id,
		`sync_cursor = $2, last_sync_at = now(), status = 'active', last_error = NULL`, cursor)
}

// SaveFailure implements [repository.MailConnectionRepository].
func (r *mailConnectionRepo) SaveFailure(
	ctx context.Context,
	id uuid.UUID,
	status model.MailConnectionStatus,
	lastError string,
) error {
	return r.update(ctx, "save failure", id, `status = $2, last_error = NULLIF($3, '')`, string(status), lastError)
}

// update applies set to one live connection; set may use $2 onward.
func (r *mailConnectionRepo) update(ctx context.Context, action string, id uuid.UUID, set string, args ...any) error {
	tag, err := r.Executor(ctx).Exec(ctx,
		`UPDATE mail_connections SET `+set+`, updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		append([]any{id}, args...)...)
	if err != nil {
		return errs.Wrap(errs.CodeConnectionUpdateFailed, action, err, errs.WithDetail("connection_id", id))
	}
	if tag.RowsAffected() == 0 {
		return errs.NewWith(errs.CodeConnectionNotFound, action+": connection not found",
			errs.WithDetail("connection_id", id))
	}
	return nil
}

func (r *mailConnectionRepo) list(
	ctx context.Context,
	action, sql string,
	args ...any,
) ([]model.MailConnection, error) {
	rows, err := r.Executor(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, errs.Wrap(errs.CodeConnectionLookupFailed, action, err)
	}
	defer rows.Close()

	var out []model.MailConnection
	for rows.Next() {
		c, err := scanMailConnection(rows)
		if err != nil {
			return nil, errs.Wrap(errs.CodeConnectionLookupFailed, action, err)
		}
		out = append(out, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeConnectionLookupFailed, action, err)
	}
	return out, nil
}

// lookupResult maps a single-row miss to CONNECTION_NOT_FOUND (still
// matching repository.ErrNotFound through the wrapped cause).
func mailConnectionResult(
	c *model.MailConnection,
	err error,
	action string,
	details errs.Option,
) (*model.MailConnection, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.Wrap(errs.CodeConnectionNotFound, action+": connection not found", err, details)
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeConnectionLookupFailed, action, err, details)
	}
	return c, nil
}

func scanMailConnection(row pgx.Row) (*model.MailConnection, error) {
	var (
		c       model.MailConnection
		status  string
		lastErr *string
	)
	err := row.Scan(
		&c.ID, &c.Provider, &c.Account, &c.BudgetIDs, &c.Credentials, &c.Cursor,
		&c.WatchExpiry, &status, &c.LastSyncAt, &lastErr, &c.PausedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.Status = model.MailConnectionStatus(status)
	if lastErr != nil {
		c.LastError = *lastErr
	}
	if c.BudgetIDs == nil {
		c.BudgetIDs = []uuid.UUID{}
	}
	return &c, nil
}

func normalizeMail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
