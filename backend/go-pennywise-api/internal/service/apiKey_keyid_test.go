package service

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var keyIDPattern = regexp.MustCompile(`^k_[0-9a-f]{12}$`)

func TestAPIKeyService_GenerateReturnsAPublicKeyID(t *testing.T) {
	svc := NewApiKeyService(nil)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		fullKey, keyID, err := svc.Generate()
		require.NoError(t, err)
		require.Regexp(t, keyIDPattern, keyID)
		require.False(t, seen[keyID], "key ids are unique")
		seen[keyID] = true

		_, _, secret, err := svc.ParseKey(fullKey)
		require.NoError(t, err)
		assert.NotContains(t, secret, strings.TrimPrefix(keyID, "k_"), "the id is not a piece of the secret")
	}
}

// Every key used to be stored with key_id = "", so the second key ever created
// violated UNIQUE (key_id).
func TestAPIKeyService_CreateStoresDistinctKeyIDs(t *testing.T) {
	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	ctx := utils.WithUserID(context.Background(), uuid.New())
	var stored []*model.APIKey
	repo.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		stored = append(stored, args.Get(1).(*model.APIKey))
	})

	key1, err := svc.Create(ctx, &model.APIKey{Name: "insomnia-local"})
	require.NoError(t, err)
	key2, err := svc.Create(ctx, &model.APIKey{Name: "insomnia-local"})
	require.NoError(t, err)

	require.Len(t, stored, 2)
	assert.Regexp(t, keyIDPattern, stored[0].KeyID)
	assert.Regexp(t, keyIDPattern, stored[1].KeyID)
	assert.NotEqual(t, stored[0].KeyID, stored[1].KeyID)
	assert.Equal(t, utils.Hash(key1), stored[0].HashedKey)
	assert.Equal(t, utils.Hash(key2), stored[1].HashedKey)
}

// The random part of a key is the secret; it must not reach the logs, at any level.
func TestAPIKeyService_NeverLogsTheSecret(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	repo.On("GetByHash", mock.Anything, mock.Anything).Return(&model.APIKey{}, nil)
	fullKey, _, err := svc.Generate()
	require.NoError(t, err)
	_, _, secret, _ := svc.ParseKey(fullKey)

	assert.True(t, svc.ValidateFormat(fullKey))
	_, err = svc.GetByHash(context.Background(), fullKey)
	require.NoError(t, err)

	assert.NotContains(t, buf.String(), secret)
	assert.NotContains(t, buf.String(), fullKey)
}
