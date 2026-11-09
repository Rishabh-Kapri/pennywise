package postgres

import (
	"context"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	sharedsql "github.com/Rishabh-Kapri/pennywise/backend/shared/migrations/sql"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuthUser(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := testPool(t).Exec(ctx, `INSERT INTO auth_users (id) VALUES ($1)`, id)
	require.NoError(t, err)
	return id
}

func apiKeyRow(userID uuid.UUID, keyID string) *model.APIKey {
	expiry := time.Now().Add(time.Hour)
	return &model.APIKey{
		KeyID: keyID, HashedKey: "hash-" + uuid.NewString(), Name: "k", UserID: userID,
		Scopes: []model.Scope{model.ScopeRead}, RateLimit: 10, IsActive: true, ExpiresAt: &expiry,
	}
}

func TestAPIKeyRepo_KeyIDsAreUniqueAndLookupable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewAPIKeyRepository(pool)
	user := newAuthUser(t, ctx)

	require.NoError(t, repo.Create(ctx, apiKeyRow(user, "k_"+uuid.NewString()[:12])))
	second := apiKeyRow(user, "k_"+uuid.NewString()[:12])
	require.NoError(t, repo.Create(ctx, second), "a second key for the same user is allowed")

	got, err := repo.GetByKeyID(ctx, second.KeyID)
	require.NoError(t, err)
	assert.Equal(t, second.HashedKey, got.HashedKey)
	assert.Equal(t, second.KeyID, got.KeyID)
	assert.Equal(t, user, got.UserID)
	assert.True(t, got.IsActive)

	byHash, err := repo.GetByHash(ctx, second.HashedKey)
	require.NoError(t, err, "the lookup API-key authentication uses")
	assert.Equal(t, second.KeyID, byHash.KeyID)
	assert.Equal(t, []model.Scope{model.ScopeRead}, byHash.Scopes)

	_, err = repo.GetByKeyID(ctx, "k_000000000000")
	assert.ErrorIs(t, err, repository.ErrNotFound)

	require.NoError(t, repo.UpdateLastUsed(ctx, got.ID))
	all, err := repo.GetAll(ctx, user)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	dup := apiKeyRow(user, second.KeyID)
	assert.Error(t, repo.Create(ctx, dup), "key ids stay unique")
}

func TestAPIKeyRepo_EmptyKeyIDIsRejected(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	err := NewAPIKeyRepository(pool).Create(ctx, apiKeyRow(newAuthUser(t, ctx), ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_keys_key_id_not_empty")
}

// 00012 repairs databases that already hold a key with key_id = ”.
func TestMigration00012_BackfillsEmptyKeyIDs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	user := newAuthUser(t, ctx)

	body, err := fs.ReadFile(sharedsql.Migrations, path.Join(sharedsql.MigrationsDir, "00012_api_key_ids.sql"))
	require.NoError(t, err)
	up := strings.SplitN(strings.SplitN(string(body), "-- +goose Down", 2)[0], "-- +goose Up", 2)[1]

	// Recreate the broken state: drop the constraint, store a key the old way.
	_, err = pool.Exec(ctx, `ALTER TABLE api_keys DROP CONSTRAINT api_keys_key_id_not_empty`)
	require.NoError(t, err)
	broken := apiKeyRow(user, "")
	require.NoError(t, NewAPIKeyRepository(pool).Create(ctx, broken))

	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var keyID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT key_id FROM api_keys WHERE hashed_key = $1`, broken.HashedKey).Scan(&keyID))
	assert.Regexp(t, regexp.MustCompile(`^k_[0-9a-f]{12}$`), keyID)
	var empty int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM api_keys WHERE key_id = ''`).Scan(&empty))
	assert.Zero(t, empty)
	assert.Error(t, NewAPIKeyRepository(pool).Create(ctx, apiKeyRow(user, "")), "the constraint is back")
}

// 00013 renames the pre-squash column spelling; on a fresh database it is a no-op.
func TestMigration00013_RenamesTheOldReferrerColumn(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	body, err := fs.ReadFile(sharedsql.Migrations, path.Join(sharedsql.MigrationsDir, "00013_api_key_allowed_referrers.sql"))
	require.NoError(t, err)
	up := strings.SplitN(strings.SplitN(string(body), "-- +goose Down", 2)[0], "-- +goose Up", 2)[1]
	column := func() string {
		var name string
		require.NoError(t, pool.QueryRow(ctx, `SELECT column_name FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'api_keys' AND column_name LIKE 'allowed_refer%'`).Scan(&name))
		return name
	}

	_, err = pool.Exec(ctx, up)
	require.NoError(t, err, "no-op on a database that already has the right name")
	assert.Equal(t, "allowed_referrers", column())

	_, err = pool.Exec(ctx, `ALTER TABLE api_keys RENAME COLUMN allowed_referrers TO allowed_referers`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	assert.Equal(t, "allowed_referrers", column(), "an old database is brought in line")
	require.NoError(t, NewAPIKeyRepository(pool).Create(ctx, apiKeyRow(newAuthUser(t, ctx), "k_"+uuid.NewString()[:12])))
}
