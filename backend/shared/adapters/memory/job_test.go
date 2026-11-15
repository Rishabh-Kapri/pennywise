package memory

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func claimOne(t *testing.T, b job.Broker, worker string) (*job.Job, job.Claim) {
	t.Helper()
	j, err := b.ClaimJob(context.Background(), []string{"q"}, worker)
	require.NoError(t, err)
	return j, job.Claim{JobID: j.ID, WorkerID: worker, Attempt: j.Attempt}
}

func enqueue(t *testing.T, b job.Broker, opts ...job.EnqueueOption) job.EnqueueResult {
	t.Helper()
	res, err := b.Enqueue(context.Background(), "t", []byte(`{}`), append([]job.EnqueueOption{job.WithQueue("q")}, opts...)...)
	require.NoError(t, err)
	return res
}

func requireUnavailable(t *testing.T, b job.Broker) {
	t.Helper()
	_, err := b.ClaimJob(context.Background(), []string{"q"}, "w")
	require.True(t, errs.HasCode(err, errs.CodeJobsUnavailable), "got %v", err)
}

func TestEnqueueDedupes(t *testing.T) {
	b := NewMemoryJobBrokerAdapter()
	first := enqueue(t, b, job.WithIdempotencyKey("k"))
	again := enqueue(t, b, job.WithIdempotencyKey("k"))
	require.True(t, again.Duplicate)
	require.Equal(t, first.JobID, again.JobID)

	other := enqueue(t, b, job.WithIdempotencyKey("other"))
	require.False(t, other.Duplicate)

	schedule, at := uuid.New(), time.Now()
	tick := enqueue(t, b, job.WithSchedule(schedule, at))
	require.True(t, enqueue(t, b, job.WithSchedule(schedule, at)).Duplicate)
	require.Equal(t, tick.JobID, enqueue(t, b, job.WithSchedule(schedule, at)).JobID)
	require.False(t, enqueue(t, b, job.WithSchedule(schedule, at.Add(time.Minute))).Duplicate)
}

func TestClaimIsExclusiveAndOnlyForItsQueues(t *testing.T) {
	b := NewMemoryJobBrokerAdapter()
	enqueue(t, b)

	_, err := b.ClaimJob(context.Background(), []string{"another"}, "w")
	require.True(t, errs.HasCode(err, errs.CodeJobsUnavailable))

	j, _ := claimOne(t, b, "w1")
	require.Equal(t, 1, j.Attempt)
	requireUnavailable(t, b)
}

func TestFailedAttemptRunsAgainAtRunAt(t *testing.T) {
	b := NewMemoryJobBrokerAdapter()
	enqueue(t, b)
	j, claim := claimOne(t, b, "w")

	require.NoError(t, b.MarkPending(context.Background(), j, claim, "boom", time.Now().Add(time.Hour)))
	requireUnavailable(t, b)

	// Not owned any more: a second report from the same attempt is rejected.
	err := b.MarkPending(context.Background(), j, claim, "boom", time.Now())
	require.True(t, errs.HasCode(err, errs.CodeJobClaimLost))
}

func TestFailedAttemptDueNowIsClaimedAgain(t *testing.T) {
	b := NewMemoryJobBrokerAdapter()
	enqueue(t, b)
	j, claim := claimOne(t, b, "w")
	require.NoError(t, b.MarkPending(context.Background(), j, claim, "boom", time.Now()))

	again, _ := claimOne(t, b, "w")
	require.Equal(t, 2, again.Attempt)
}

func TestDeadAndCompletedJobsAreNotClaimedAgain(t *testing.T) {
	for name, finish := range map[string]func(job.Broker, job.Claim) error{
		"dead": func(b job.Broker, c job.Claim) error { return b.MarkDead(context.Background(), c, "boom", "why") },
		"completed": func(b job.Broker, c job.Claim) error {
			return b.MarkCompleted(context.Background(), c, json.RawMessage(`{}`), time.Second)
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := NewMemoryJobBrokerAdapter()
			enqueue(t, b)
			_, claim := claimOne(t, b, "w")
			require.NoError(t, finish(b, claim))
			requireUnavailable(t, b)
			require.True(t, errs.HasCode(finish(b, claim), errs.CodeJobClaimLost), "finishing twice is a lost claim")
		})
	}
}

func TestOnlyTheOwningAttemptMayReport(t *testing.T) {
	b := NewMemoryJobBrokerAdapter()
	enqueue(t, b)
	_, claim := claimOne(t, b, "w1")

	stranger := claim
	stranger.WorkerID = "w2"
	require.True(t, errs.HasCode(b.Heartbeat(context.Background(), stranger), errs.CodeJobClaimLost))
	require.True(t, errs.HasCode(b.UpdateProgress(context.Background(), stranger, json.RawMessage(`{}`)), errs.CodeJobClaimLost))
	require.NoError(t, b.Heartbeat(context.Background(), claim))
	require.NoError(t, b.UpdateProgress(context.Background(), claim, json.RawMessage(`{"current":1}`)))
}

func TestRequeueStaleRunning(t *testing.T) {
	ctx := context.Background()

	t.Run("a job with no recent sign of life goes back to queued", func(t *testing.T) {
		b := NewMemoryJobBrokerAdapter()
		enqueue(t, b, func(o *job.EnqueueOptions) { o.MaxAttempts = 2 })
		_, claim := claimOne(t, b, "w")

		n, err := b.RequeueStaleRunning(ctx, time.Hour, "reaper")
		require.NoError(t, err)
		require.Zero(t, n, "a fresh claim is not stale")

		time.Sleep(5 * time.Millisecond)
		n, err = b.RequeueStaleRunning(ctx, time.Millisecond, "reaper")
		require.NoError(t, err)
		require.EqualValues(t, 1, n)
		require.True(t, errs.HasCode(b.Heartbeat(ctx, claim), errs.CodeJobClaimLost), "the old worker lost its claim")

		again, _ := claimOne(t, b, "w2")
		require.Equal(t, 2, again.Attempt)
	})

	t.Run("a heartbeat keeps the job alive", func(t *testing.T) {
		b := NewMemoryJobBrokerAdapter()
		enqueue(t, b)
		_, claim := claimOne(t, b, "w")
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, b.Heartbeat(ctx, claim))

		n, err := b.RequeueStaleRunning(ctx, 10*time.Millisecond, "reaper")
		require.NoError(t, err)
		require.Zero(t, n)
	})

	t.Run("a job out of attempts dies instead", func(t *testing.T) {
		b := NewMemoryJobBrokerAdapter()
		enqueue(t, b, func(o *job.EnqueueOptions) { o.MaxAttempts = 1 })
		claimOne(t, b, "w")
		time.Sleep(5 * time.Millisecond)

		n, err := b.RequeueStaleRunning(ctx, time.Millisecond, "reaper")
		require.NoError(t, err)
		require.EqualValues(t, 1, n)
		requireUnavailable(t, b)
	})
}
