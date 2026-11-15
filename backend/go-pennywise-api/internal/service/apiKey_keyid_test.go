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

// Every key used to be stored with key_id = "", so the second key ever created
// violated UNIQUE (key_id). A key must also pass the format check the auth
// adapter runs, and its public id must not be a piece of the secret.
func TestAPIKeyService_CreateIssuesUsableDistinctKeys(t *testing.T) {
	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	ctx := utils.WithUserID(context.Background(), uuid.New())
	var stored []*model.APIKey
	repo.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		stored = append(stored, args.Get(1).(*model.APIKey))
	})

	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		fullKey, err := svc.Create(ctx, &model.APIKey{Name: "insomnia-local"})
		require.NoError(t, err)
		require.True(t, model.ValidAPIKeyFormat(fullKey))

		saved := stored[len(stored)-1]
		require.Regexp(t, keyIDPattern, saved.KeyID)
		require.False(t, seen[saved.KeyID], "key ids are unique")
		seen[saved.KeyID] = true
		assert.Equal(t, utils.Hash(fullKey), saved.HashedKey)

		_, _, secret, _ := model.ParseAPIKey(fullKey)
		assert.NotContains(t, secret, strings.TrimPrefix(saved.KeyID, "k_"))
	}
}

// The random part of a key is the secret; it must not reach the logs, at any level.
func TestAPIKeyService_NeverLogsTheSecret(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	ctx := utils.WithUserID(context.Background(), uuid.New())

	fullKey, err := svc.Create(ctx, &model.APIKey{Name: "k"})
	require.NoError(t, err)
	_, _, secret, _ := model.ParseAPIKey(fullKey)

	assert.NotContains(t, buf.String(), secret)
	assert.NotContains(t, buf.String(), fullKey)
}
