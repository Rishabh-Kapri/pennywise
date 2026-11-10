// This package gives a unified methods to run the jobs in the services
package jobrunner

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"os"

	// "errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	jobPort "github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// separate timeout for marking a job dead
const persistTimeout = 10 * time.Second

type HandlerFunc func(ctx context.Context, job *jobPort.Job) (json.RawMessage, error)

type workerOpts struct {
	concurrency       int
	prefix            string
	heartbeatInterval time.Duration
	reaperInterval    time.Duration
	staleThreshold    time.Duration
	shutdownTimeout   time.Duration
	baseBackoff       time.Duration
	maxBackoff        time.Duration
	queues            []string
}
type workerOpt func(*workerOpts)

type jobRegistry struct {
	handlers map[string]HandlerFunc
}

type Worker struct {
	broker   jobPort.Broker
	registry jobRegistry
	config   workerOpts
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func defaultWorkerOpts() workerOpts {
	return workerOpts{
		concurrency:       5,
		heartbeatInterval: 30 * time.Second,
		reaperInterval:    30 * time.Second,
		staleThreshold:    5 * time.Minute,
		prefix:            "pw",
		shutdownTimeout:   time.Second * 30,
		baseBackoff:       time.Second * 10,
		maxBackoff:        time.Minute * 5,
	}
}

// WithQueue sets the queue the worker claims from (a model.Queue*
// constant). Required: a worker only ever sees jobs on its own queue.
func WithQueues(q []string) workerOpt {
	return func(wo *workerOpts) {
		wo.queues = q
	}
}

// WithConcurrency sets the concurrency for handler goroutines. Default: 5
func WithConcurrency(c int) workerOpt {
	return func(wo *workerOpts) {
		wo.concurrency = c
	}
}

// WithPrefix sets the worker prefix. Default: pw (pennywise)
func WithPrefix(p string) workerOpt {
	return func(wo *workerOpts) {
		wo.prefix = p
	}
}

// WithShutdownTimeout sets the timeout for worker after context is cancelled. Default: 30s
func WithShutdownTimeout(t time.Duration) workerOpt {
	return func(wo *workerOpts) {
		wo.shutdownTimeout = t
	}
}

// Returns a new worker that will run the job
func NewWorker(broker jobPort.Broker, opts ...workerOpt) (*Worker, error) {
	workerOpts := defaultWorkerOpts()
	for _, opt := range opts {
		opt(&workerOpts)
	}
	if len(workerOpts.queues) == 0 {
		return nil, errs.New(errs.CodeJobInvalidOption, "worker needs a queue (jobrunner.WithQueue)")
	}
	logger.Logger(context.Background()).Info("opts", "queues", workerOpts.queues,
		"prefix", workerOpts.prefix, "concurrency", workerOpts.concurrency)
	return &Worker{
		broker:   broker,
		registry: jobRegistry{handlers: map[string]HandlerFunc{}},
		config:   workerOpts,
		wg:       sync.WaitGroup{},
	}, nil
}

// overrides an existing handler for a jobType
func (w *Worker) RegisterJobHandler(jobType string, h HandlerFunc) {
	w.registry.handlers[jobType] = h
}

// Starts the loop
func (w *Worker) Run(ctx context.Context) error {
	runCtx, runCtxCancel := context.WithCancel(ctx)
	// start the worker pool
	w.poolStart(runCtx)

	// check for original context to be cancelled, this is blocking
	select {
	case <-ctx.Done():
	case <-w.stopCh:
	}
	// this is needed so that w.stopCh branch works
	// otherwise calling Stop() never cancels the context
	// and ctx.Done() is never called for worker goroutines
	runCtxCancel()

	// separate context for shutdown
	waitCtx, cancel := context.WithTimeout(context.Background(), w.config.shutdownTimeout)
	defer cancel()

	err := w.poolWait(waitCtx)
	return err
}

// Request a graceful shutdown
func (w *Worker) Stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
}

func instanceID() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

// handler panic in handler code
func (w *Worker) runJobHandler(
	ctx context.Context,
	job *jobPort.Job,
	handler HandlerFunc,
) (result json.RawMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errs.New(errs.CodeJobHandlerError, "handler panicked: %v", r)
			logger.Logger(ctx).Error("handler panicked", "panic", r, "stack", string(debug.Stack()))
		}
	}()
	ctx = job.Metadata.Context(ctx)
	return handler(ctx, job)
}

func (w *Worker) handleFailure(ctx context.Context, job *jobPort.Job, claim jobPort.Claim, jobErr error) error {
	var err error
	if job.Attempt >= job.MaxAttempts {
		err = w.broker.MarkDead(ctx, claim, jobErr.Error(), "all attempts exhausted")
	} else {
		backoff := w.computeBackoff(job.Attempt)
		nextRunAt := time.Now().Add(backoff)
		err = w.broker.MarkPending(ctx, job, claim, jobErr.Error(), nextRunAt)
	}
	if errs.HasCode(err, errs.CodeJobClaimLost) {
		logger.Logger(ctx).Warn("lost job claim before persisting failure attempt", "claim", claim)
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func (w *Worker) startHeartbeat(heartbeatCtx context.Context, claim jobPort.Claim) func() {
	heartbeatCtx, cancel := context.WithCancel(heartbeatCtx)

	go func() {
		ticker := time.NewTicker(w.config.heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				err := w.broker.Heartbeat(heartbeatCtx, claim)
				if err == nil {
					continue
				}
				if errs.HasCode(err, errs.CodeJobClaimLost) {
					logger.Logger(heartbeatCtx).Warn("lost claim of job before heartbeat")
					cancel()
					return
				}
				logger.Logger(heartbeatCtx).Warn("heartbeat failed, retyring on next tick", "err", err)
			}
		}
	}()

	return cancel
}

func (w *Worker) execute(ctx context.Context, workerID string, job *jobPort.Job) error {
	handler, ok := w.registry.handlers[job.Type]
	claim := jobPort.Claim{
		JobID:    job.ID,
		WorkerID: workerID,
		Attempt:  job.Attempt,
		BudgetID: job.BudgetID,
		Type:     job.Type,
	}
	if !ok {
		logger.Logger(ctx).Warn("no handler registered for job", "type", job.Type)
		persistCtx, persistCancel := context.WithTimeout(context.Background(), persistTimeout)
		defer persistCancel()
		err := w.broker.MarkDead(
			persistCtx,
			claim,
			fmt.Sprintf("no handler registered for type: %q", job.Type),
			"no handler registered",
		)
		if err != nil && !errs.HasCode(err, errs.CodeJobClaimLost) {
			return err
		}
		return nil
	}

	stopHearbeat := w.startHeartbeat(ctx, claim)
	defer stopHearbeat()

	jobStartTime := time.Now()
	result, err := w.runJobHandler(withProgress(ctx, w.broker, claim), job, handler)
	if err != nil {
		return w.handleFailure(ctx, job, claim, err)
	}

	duration := time.Since(jobStartTime)

	persistCtx, persistCancel := context.WithTimeout(context.Background(), persistTimeout)
	defer persistCancel()
	if err = w.broker.MarkCompleted(persistCtx, claim, result, duration); err != nil {
		if errs.HasCode(err, errs.CodeJobClaimLost) {
			logger.Logger(ctx).Warn("lost task ownership before MarkCompleted")
			return nil
		}
		// return w.handleFailure(ctx, job, claim, err)
		return errs.Wrap(errs.CodeJobUpdateFailed, "error while marking job complete", err)
	}
	logger.Logger(ctx).Info("job completed", "duration_s", duration.Seconds())
	return nil
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func (w *Worker) poolStart(ctx context.Context) {
	log := logger.Logger(ctx)
	log.Info(
		"starting worker pool",
		"queue",
		w.config.queues,
		"concurrency",
		w.config.concurrency,
		"prefix",
		w.config.prefix,
	)

	for i := 0; i < w.config.concurrency; i++ {
		workerID := fmt.Sprintf("%s-%s-%d", w.config.prefix, instanceID(), i+1)
		log.Info("worker", "id", workerID)
		w.wg.Go(func() {
			for {
				job, err := w.broker.ClaimJob(ctx, w.config.queues, workerID)
				if err == nil {
					log.Info("got a job", "job", job, "worker", workerID, "err", err)
					if jobErr := w.execute(ctx, workerID, job); jobErr != nil {
						log.Error("execute job failed", "err", jobErr)
					}
					continue
				}
				if ctx.Err() != nil {
					return
				}
				if errs.HasCode(err, errs.CodeJobsUnavailable) {
					// no jobs available
					sleep(ctx, w.config.baseBackoff)
					continue
				}
				log.Error("claim failed, will retry", "err", err)
				backoff := w.computeBackoff(3)
				sleep(ctx, backoff)
				continue
			}
		})
	}

	reaperID := fmt.Sprintf("%s-reaper", w.config.prefix)
	w.wg.Go(func() {
		newReaper(
			reaperID,
			w.broker,
			w.config.reaperInterval,
			w.config.staleThreshold,
		).start(ctx)
	})
}

func (w *Worker) poolWait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.wg.Wait()
	}()

	select {
	case <-ctx.Done(): // timeout reached
		return ctx.Err()
	case <-done: // all workers finished
		return nil
	}
}

func (w *Worker) computeBackoff(attempts int) time.Duration {
	base := float64(w.config.baseBackoff)
	cap := float64(w.config.maxBackoff)
	attempt := float64(max(attempts-1, 0))

	exp := math.Min(cap, base*math.Pow(2, attempt))
	// used to prevent all workers retyring at the exact same time
	jitter := rand.Float64() * exp
	backoff := time.Duration(jitter)
	return backoff
}
