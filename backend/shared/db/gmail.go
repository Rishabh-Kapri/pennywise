package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GmailRepository interface {
	ListConnections(ctx context.Context, userID uuid.UUID) ([]model.GoogleProviderUser, error)
	GetConnection(ctx context.Context, userID uuid.UUID, googleID string, clientType model.GoogleOAuthClientType) (*model.GoogleProviderUser, error)
	SetPaused(ctx context.Context, userID uuid.UUID, googleID string, paused bool) error
	UpdateWatch(ctx context.Context, userID uuid.UUID, googleID string, clientType model.GoogleOAuthClientType, historyID uint64, expiryAt int64) error
}

type gmailRepo struct{ BaseRepository }

func NewGmailRepository(pool *pgxpool.Pool) GmailRepository {
	return &gmailRepo{BaseRepository: NewBaseRepository(pool)}
}

const gmailConnectionQuery = `
	SELECT gpu.id, gpu.oauth_client_type, gpu.email, gpu.name, gpu.picture,
	       gpu.refresh_token, gpu.gmail_history_id, gpu.last_gmail_sync,
	       gpu.expiry_at, gpu.gmail_ingestion_paused
	FROM google_provider_users gpu
	JOIN auth_providers ap ON ap.provider_id = gpu.id
	    AND ap.oauth_client_type = gpu.oauth_client_type AND ap.provider_type = 'google'
	JOIN auth_users au ON au.id = ap.auth_user_id
	WHERE ap.auth_user_id = $1 AND ap.deleted = FALSE AND gpu.deleted = FALSE AND au.deleted = FALSE`

func scanGmailConnection(row pgx.Row) (*model.GoogleProviderUser, error) {
	var user model.GoogleProviderUser
	err := row.Scan(&user.ID, &user.OAuthClientType, &user.Email, &user.Name, &user.Picture,
		&user.RefreshToken, &user.GmailHistoryID, &user.LastGmailSync, &user.ExpiryAt, &user.GmailIngestionPaused)
	return &user, err
}

func (r *gmailRepo) ListConnections(ctx context.Context, userID uuid.UUID) ([]model.GoogleProviderUser, error) {
	rows, err := r.Executor(nil).Query(ctx, gmailConnectionQuery+` ORDER BY gpu.email, gpu.oauth_client_type`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]model.GoogleProviderUser, 0)
	for rows.Next() {
		user, err := scanGmailConnection(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

func (r *gmailRepo) GetConnection(ctx context.Context, userID uuid.UUID, googleID string, clientType model.GoogleOAuthClientType) (*model.GoogleProviderUser, error) {
	return scanGmailConnection(r.Executor(nil).QueryRow(ctx, gmailConnectionQuery+`
		AND gpu.id = $2 AND gpu.oauth_client_type = $3`, userID, googleID, clientType))
}

// Gmail's watch is mailbox-wide. Pause every OAuth client linked to this user's
// Google identity so another client cannot restart ingestion for that mailbox.
func (r *gmailRepo) SetPaused(ctx context.Context, userID uuid.UUID, googleID string, paused bool) error {
	tag, err := r.Executor(nil).Exec(ctx, `
		UPDATE google_provider_users gpu
		SET gmail_ingestion_paused = $3,
		    expiry_at = CASE WHEN $3 THEN NULL ELSE expiry_at END, updated_at = now()
		FROM auth_providers ap
		WHERE ap.provider_id = gpu.id AND ap.oauth_client_type = gpu.oauth_client_type
		    AND ap.provider_type = 'google' AND ap.auth_user_id = $1 AND gpu.id = $2
		    AND ap.deleted = FALSE AND gpu.deleted = FALSE`, userID, googleID, paused)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

// A renewed watch must not replace the ingestion cursor: doing so would skip
// mail received while paused or during a failed push delivery.
func (r *gmailRepo) UpdateWatch(ctx context.Context, userID uuid.UUID, googleID string, clientType model.GoogleOAuthClientType, historyID uint64, expiryAt int64) error {
	tag, err := r.Executor(nil).Exec(ctx, `
		UPDATE google_provider_users gpu
		SET gmail_history_id = COALESCE(gpu.gmail_history_id, $4), expiry_at = $5, updated_at = now()
		FROM auth_providers ap
		WHERE ap.provider_id = gpu.id AND ap.oauth_client_type = gpu.oauth_client_type
		    AND ap.provider_type = 'google' AND ap.auth_user_id = $1
		    AND gpu.id = $2 AND gpu.oauth_client_type = $3
		    AND ap.deleted = FALSE AND gpu.deleted = FALSE`, userID, googleID, clientType, historyID, expiryAt)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
