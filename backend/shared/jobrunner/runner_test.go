package jobrunner

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/memory"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/stretchr/testify/require"
)

const testQueue = "q"

// newTestWorker is a worker on the in-memory broker with intervals short
// enough for a test.
func newTestWorker(t *testing.T, broker job.Broker) *Worker {
	t.Helper()
	w, err := NewWorker(broker, WithQueues([]string{testQueue}), WithConcurrency(1))
	require.NoError(t, err)
	w.config.pollInterval = time.Millisecond
	w.config.baseBackoff = time.Millisecond
	w.config.maxBackoff = 5 * time.Millisecond
	w.config.heartbeatInterval = 10 * time.Millisecond
	w.config.reaperInterval = time.Hour
	return w
}

func enqueue(t *testing.T, b job.Broker, maxAttempts int) {
	t.Helper()
	_, err := b.Enqueue(context.Background(), "t", []byte(`{}`), job.WithQueue(testQueue),
		func(o *job.EnqueueOptions) { o.MaxAttempts = maxAttempts })
	require.NoError(t, err)
}

// run starts the worker and returns a func that stops it and waits for it.
func run(t *testing.T, w *Worker) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	return func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not stop")
		}
	}
}

func eventually(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	require.Eventually(t, cond, 5*time.Second, 5*time.Millisecond, msg)
}

func claimable(b job.Broker) bool {
	_, err := b.ClaimJob(context.Background(), []string{testQueue}, "probe")
	return err == nil
}

func TestWorkerRunsAJobOnce(t *testing.T) {
	broker := memory.NewMemoryJobBrokerAdapter()
	w := newTestWorker(t, broker)
	var runs atomic.Int32
	w.RegisterJobHandler("t", func(context.Context, *job.Job) (json.RawMessage, error) {
		runs.Add(1)
		return json.RawMessage(`{}`), nil
	})
	enqueue(t, broker, 3)
	stop := run(t, w)
	eventually(t, func() bool { return runs.Load() == 1 }, "handler runs")
	time.Sleep(30 * time.Millisecond)
	stop()

	require.EqualValues(t, 1, runs.Load(), "a finished job is not run again")
	require.False(t, claimable(broker))
}

func TestWorkerRetriesAFailingJobUntilItsAttemptsRunOut(t *testing.T) {
	broker := memory.NewMemoryJobBrokerAdapter()
	w := newTestWorker(t, broker)
	var runs atomic.Int32
	w.RegisterJobHandler("t", func(context.Context, *job.Job) (json.RawMessage, error) {
		runs.Add(1)
		return nil, errors.New("boom")
	})
	enqueue(t, broker, 3)
	stop := run(t, w)
	eventually(t, func() bool { return runs.Load() == 3 }, "three attempts")
	time.Sleep(50 * time.Millisecond)
	stop()

	require.EqualValues(t, 3, runs.Load(), "no attempt past MaxAttempts")
	require.False(t, claimable(broker), "the job is dead, not queued")
}

func TestWorkerFailsAJobWithNoHandler(t *testing.T) {
	broker := memory.NewMemoryJobBrokerAdapter()
	w := newTestWorker(t, broker)
	enqueue(t, broker, 3)
	stop := run(t, w)
	time.Sleep(50 * time.Millisecond)
	stop()
	require.False(t, claimable(broker), "an unhandled job is dead, not retried forever")
}

func TestWorkerTurnsAPanicIntoAFailedAttempt(t *testing.T) {
	broker := memory.NewMemoryJobBrokerAdapter()
	w := newTestWorker(t, broker)
	var runs atomic.Int32
	w.RegisterJobHandler("t", func(context.Context, *job.Job) (json.RawMessage, error) {
		if runs.Add(1) == 1 {
			panic("oops")
		}
		return nil, nil
	})
	enqueue(t, broker, 3)
	stop := run(t, w)
	eventually(t, func() bool { return runs.Load() == 2 }, "retry after the panic")
	stop()
}

// ctxStrictBroker refuses outcome writes made with a cancelled ctx, as a
// database driver does.
type ctxStrictBroker struct{ job.Broker }

func (b ctxStrictBroker) MarkPending(ctx context.Context, j *job.Job, c job.Claim, lastError string, runAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return b.Broker.MarkPending(ctx, j, c, lastError, runAt)
}

func (b ctxStrictBroker) MarkDead(ctx context.Context, c job.Claim, lastError, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return b.Broker.MarkDead(ctx, c, lastError, reason)
}

// Shutting down while a handler is mid-flight must not leave its job marked
// running: the outcome is written even though the worker's ctx is cancelled.
func TestShutdownReleasesTheRunningJob(t *testing.T) {
	broker := ctxStrictBroker{memory.NewMemoryJobBrokerAdapter()}
	w := newTestWorker(t, broker)
	started := make(chan struct{})
	w.RegisterJobHandler("t", func(ctx context.Context, _ *job.Job) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	enqueue(t, broker, 3)
	stop := run(t, w)
	<-started
	stop()

	eventually(t, func() bool { return claimable(broker) }, "the job is queued again, not stuck running")
}

// A lost claim means another worker may own the job: the handler is told to stop.
func TestHandlerIsCancelledWhenItsClaimIsLost(t *testing.T) {
	broker := memory.NewMemoryJobBrokerAdapter()
	w := newTestWorker(t, broker)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	var runs atomic.Int32
	w.RegisterJobHandler("t", func(ctx context.Context, _ *job.Job) (json.RawMessage, error) {
		if runs.Add(1) > 1 {
			return nil, nil
		}
		close(started)
		<-ctx.Done()
		close(cancelled)
		return nil, ctx.Err()
	})
	enqueue(t, broker, 3)
	stop := run(t, w)
	defer stop()
	<-started

	// The reaper's view: no sign of life for longer than the threshold.
	n, err := broker.RequeueStaleRunning(context.Background(), 0, "reaper")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler kept running after its claim was lost")
	}
}

func TestWorkerNeedsAQueue(t *testing.T) {
	_, err := NewWorker(memory.NewMemoryJobBrokerAdapter())
	require.True(t, errs.HasCode(err, errs.CodeJobInvalidOption))
}
