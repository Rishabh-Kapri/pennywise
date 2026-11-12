package postgres

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAgentUpdateConversation(t *testing.T) {
	pool := testPool(t)
	s := newSeed(t, pool)
	ctx := context.Background()
	userID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO auth_users (id) VALUES ($1)`, userID)
	require.NoError(t, err)

	repo := NewAgentRepository(pool)
	original := "New chat"
	conv, err := repo.CreateConversation(ctx, repository.CreateAgentConversationParams{
		UserID: userID, BudgetID: s.budgetID, AgentKey: "chat", Title: &original,
	})
	require.NoError(t, err)

	title := func() string {
		t.Helper()
		got, err := repo.GetConversation(ctx, conv.ID, userID, s.budgetID)
		require.NoError(t, err)
		require.NotNil(t, got.Title)
		return *got.Title
	}

	t.Run("a title is saved", func(t *testing.T) {
		renamed := "Ways I can help"
		require.NoError(t, repo.UpdateConversation(ctx, conv.ID, model.AgentConversation{Title: &renamed}))
		require.Equal(t, renamed, title())
	})
	t.Run("an update with nothing set changes nothing", func(t *testing.T) {
		before := title()
		require.NoError(t, repo.UpdateConversation(ctx, conv.ID, model.AgentConversation{}))
		require.Equal(t, before, title())
	})
	t.Run("a deleted conversation is not updated", func(t *testing.T) {
		before := title()
		require.NoError(t, repo.DeleteConversation(ctx, conv.ID, userID, s.budgetID))
		other := "after delete"
		require.NoError(t, repo.UpdateConversation(ctx, conv.ID, model.AgentConversation{Title: &other}))
		_, err := pool.Exec(ctx, `UPDATE conversations SET deleted = NULL WHERE id = $1`, conv.ID)
		require.NoError(t, err)
		require.Equal(t, before, title())
	})
}
