package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func inboundSetup(t *testing.T) (repository.InboundMessageRepository, seed) {
	t.Helper()
	pool := testPool(t)
	return NewInboundMessageRepository(pool), newSeed(t, pool)
}

func parkedReport(key model.InboundMessageKey) model.InboundMessageReport {
	confidence := 0.4
	return model.InboundMessageReport{
		Key:     key,
		Outcome: model.MessageOutcomeNeedsReview,
		Reason:  "Not sure this is a transaction.",
		Review: &model.MessageReview{
			Step: "check", Confidence: &confidence, Text: "Rs 250 debited",
			Resume: &model.MessageResume{JobType: model.JobTypeMessageExtract, Queue: model.QueueCipher, IdempotencyKey: "k"},
		},
		ReportJobID: uuid.New(),
		ReportedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}
}

func TestInboundMessageRepo_FirstReportCreatesTheRow(t *testing.T) {
	repo, s := inboundSetup(t)
	ctx := context.Background()
	key := model.InboundMessageKey{BudgetID: s.budgetID, Source: "sms", Provider: "android", ExternalID: "m1"}

	report := parkedReport(key)
	wrote, err := repo.RecordReport(ctx, report)
	require.NoError(t, err)
	assert.True(t, wrote)

	got, err := repo.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, model.InboundMessageNeedsReview, got.Status)
	assert.Equal(t, model.MessageOutcomeNeedsReview, got.Outcome)
	assert.Equal(t, report.Reason, got.Reason)
	require.NotNil(t, got.Review)
	require.NotNil(t, got.Review.Resume)
	assert.Equal(t, model.JobTypeMessageExtract, got.Review.Resume.JobType)
	assert.Equal(t, report.ReportJobID, got.ReportJobID)
	assert.Empty(t, got.TransactionIDs)

	// The same report again (a retried job) writes nothing.
	wrote, err = repo.RecordReport(ctx, report)
	require.NoError(t, err)
	assert.False(t, wrote)
}

func TestInboundMessageRepo_ReportsMoveOnlyAProcessingRow(t *testing.T) {
	repo, s := inboundSetup(t)
	ctx := context.Background()
	key := model.InboundMessageKey{BudgetID: s.budgetID, Source: "email", Provider: "gmail", ExternalID: "m2"}
	_, err := repo.RecordReport(ctx, parkedReport(key))
	require.NoError(t, err)

	created := model.InboundMessageReport{
		Key: key, Outcome: model.MessageOutcomeTransactionCreated, TransactionIDs: []uuid.UUID{uuid.New()},
		ReportJobID: uuid.New(), ReportedAt: time.Now(),
	}
	// Still waiting for a person: a stray report can't decide for them.
	wrote, err := repo.RecordReport(ctx, created)
	require.NoError(t, err)
	assert.False(t, wrote)

	// Continue, then the run reports.
	moved, err := repo.Transition(ctx, key, model.InboundMessageNeedsReview, model.InboundMessageProcessing)
	require.NoError(t, err)
	require.True(t, moved)
	got, err := repo.Get(ctx, key)
	require.NoError(t, err)
	assert.NotNil(t, got.Review, "the review stays so a failed continue can go back")

	wrote, err = repo.RecordReport(ctx, created)
	require.NoError(t, err)
	assert.True(t, wrote)
	got, err = repo.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, model.InboundMessageDone, got.Status)
	assert.Equal(t, created.TransactionIDs, got.TransactionIDs)
	assert.Nil(t, got.Review, "the report's (empty) review replaces the old one")
}

func TestInboundMessageRepo_TransitionNeedsTheExpectedStatus(t *testing.T) {
	repo, s := inboundSetup(t)
	ctx := context.Background()
	key := model.InboundMessageKey{BudgetID: s.budgetID, Source: "sms", Provider: "android", ExternalID: "m3"}
	_, err := repo.RecordReport(ctx, parkedReport(key))
	require.NoError(t, err)

	moved, err := repo.Transition(ctx, key, model.InboundMessageNeedsReview, model.InboundMessageProcessing)
	require.NoError(t, err)
	assert.True(t, moved)
	// A double click finds it already processing.
	moved, err = repo.Transition(ctx, key, model.InboundMessageNeedsReview, model.InboundMessageProcessing)
	require.NoError(t, err)
	assert.False(t, moved)

	missing := key
	missing.ExternalID = "nope"
	moved, err = repo.Transition(ctx, missing, model.InboundMessageNeedsReview, model.InboundMessageDismissed)
	require.NoError(t, err)
	assert.False(t, moved)
	_, err = repo.Get(ctx, missing)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestInboundMessageRepo_NeedsReviewRequiresAReview(t *testing.T) {
	repo, s := inboundSetup(t)
	ctx := context.Background()
	key := model.InboundMessageKey{BudgetID: s.budgetID, Source: "sms", Provider: "android", ExternalID: "m4"}
	report := parkedReport(key)
	report.Review = nil
	_, err := repo.RecordReport(ctx, report)
	assert.Error(t, err, "the table refuses a parked message nobody can act on")
}

func TestInboundMessageRepo_ListAndCountByStatus(t *testing.T) {
	repo, s := inboundSetup(t)
	ctx := context.Background()
	for i, id := range []string{"a", "b", "c"} {
		key := model.InboundMessageKey{BudgetID: s.budgetID, Source: "sms", Provider: "android", ExternalID: id}
		report := parkedReport(key)
		if i == 2 {
			report.Outcome, report.Review = model.MessageOutcomeNotTransaction, nil
		}
		_, err := repo.RecordReport(ctx, report)
		require.NoError(t, err)
	}
	other := newSeed(t, testPool(t))
	_, err := repo.RecordReport(ctx, parkedReport(model.InboundMessageKey{
		BudgetID: other.budgetID, Source: "sms", Provider: "android", ExternalID: "a",
	}))
	require.NoError(t, err)

	waiting, err := repo.List(ctx, s.budgetID, model.InboundMessageNeedsReview, 10)
	require.NoError(t, err)
	assert.Len(t, waiting, 2)
	counts, err := repo.CountByStatus(ctx, s.budgetID)
	require.NoError(t, err)
	assert.Equal(t, map[model.InboundMessageStatus]int{
		model.InboundMessageNeedsReview: 2, model.InboundMessageDone: 1,
	}, counts)
}
