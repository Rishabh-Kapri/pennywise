// Package queue is the work-queue port: services publish jobs to a named
// topic and a pool of workers consumes them. No driver types appear here;
// the adapter (shared/adapters/redis) is chosen by factory.ConnectQueue.
//
// Semantics every adapter must provide:
//
//   - A topic is one work queue. Each message goes to exactly one worker
//     (competing consumers), not to every subscriber.
//   - Delivery is at least once. A handler can see the same message twice
//     (worker crash, lost ack, visibility timeout), so handlers must be
//     idempotent; Message.ID is stable across redeliveries for deduping.
//   - A handler that returns nil acks the message. Any other error retries it
//     after ConsumeOptions.Backoff until MaxAttempts, then dead-letters it.
//     Wrap the error with Permanent to dead-letter immediately.
//   - A handler that runs past VisibilityTimeout has its ctx cancelled and the
//     attempt counts as failed, so a message that crashes or hangs workers
//     ends up dead-lettered instead of looping forever.
package queue

import (
	"context"
	"errors"
	"time"
)

// Message is one delivery of a published job.
type Message struct {
	// ID identifies the job, not the delivery: it is the same on every retry.
	ID    string
	Topic string
	Body  []byte
	// Headers carry metadata alongside the body (correlation ID, caller
	// service, ...). Never nil on delivery.
	Headers map[string]string
	// Attempt is 1 on the first delivery and increments on each retry.
	Attempt     int
	PublishedAt time.Time
}

// Handler processes one message. See the package doc for how its return
// value is treated.
type Handler func(ctx context.Context, msg Message) error

// Publisher enqueues jobs.
type Publisher interface {
	// Publish enqueues body on topic and returns the new message ID.
	Publish(ctx context.Context, topic string, body []byte, opts ...PublishOption) (string, error)
}

// Consumer runs handlers for a topic's messages.
type Consumer interface {
	// Consume blocks, running handler for each message on topic, until ctx is
	// cancelled or a non-recoverable error occurs. It returns nil after a
	// cancel, once in-flight handlers have returned. Messages interrupted by
	// the shutdown are put back without using up an attempt.
	Consume(ctx context.Context, topic string, handler Handler, opts ...ConsumeOption) error
}

// Queue is the full port. Closing the underlying connection is the
// factory's job (factory.QueueConn.Close), not the port's.
type Queue interface {
	Publisher
	Consumer
}

// PublishOptions is the resolved form of PublishOption, for adapters.
type PublishOptions struct {
	Delay   time.Duration
	Headers map[string]string
}

type PublishOption func(*PublishOptions)

// WithDelay makes the message available no earlier than d from now.
func WithDelay(d time.Duration) PublishOption {
	return func(o *PublishOptions) { o.Delay = d }
}

// WithHeaders attaches metadata to the message. Later calls add to (and
// override keys of) earlier ones.
func WithHeaders(h map[string]string) PublishOption {
	return func(o *PublishOptions) {
		if o.Headers == nil {
			o.Headers = make(map[string]string, len(h))
		}
		for k, v := range h {
			o.Headers[k] = v
		}
	}
}

// ResolvePublishOptions applies opts over the zero value.
func ResolvePublishOptions(opts ...PublishOption) PublishOptions {
	var o PublishOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// ConsumeOptions is the resolved form of ConsumeOption, for adapters.
type ConsumeOptions struct {
	// Concurrency is how many handlers run at once. Default 1.
	Concurrency int
	// MaxAttempts is the total number of deliveries before dead-lettering.
	// Default 5.
	MaxAttempts int
	// VisibilityTimeout bounds one handler run; past it the attempt fails and
	// the message can be redelivered. Default 5m.
	VisibilityTimeout time.Duration
	// Backoff returns the delay before delivery number attempt+1, given that
	// delivery number attempt failed. Default ExponentialBackoff(1s, 5m).
	Backoff func(attempt int) time.Duration
}

type ConsumeOption func(*ConsumeOptions)

func WithConcurrency(n int) ConsumeOption {
	return func(o *ConsumeOptions) { o.Concurrency = n }
}

func WithMaxAttempts(n int) ConsumeOption {
	return func(o *ConsumeOptions) { o.MaxAttempts = n }
}

func WithVisibilityTimeout(d time.Duration) ConsumeOption {
	return func(o *ConsumeOptions) { o.VisibilityTimeout = d }
}

func WithBackoff(f func(attempt int) time.Duration) ConsumeOption {
	return func(o *ConsumeOptions) { o.Backoff = f }
}

// ResolveConsumeOptions applies opts over the defaults and clamps invalid
// values back to them.
func ResolveConsumeOptions(opts ...ConsumeOption) ConsumeOptions {
	o := ConsumeOptions{
		Concurrency:       1,
		MaxAttempts:       5,
		VisibilityTimeout: 5 * time.Minute,
		Backoff:           ExponentialBackoff(time.Second, 5*time.Minute),
	}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Concurrency < 1 {
		o.Concurrency = 1
	}
	if o.MaxAttempts < 1 {
		o.MaxAttempts = 1
	}
	if o.VisibilityTimeout <= 0 {
		o.VisibilityTimeout = 5 * time.Minute
	}
	if o.Backoff == nil {
		o.Backoff = ExponentialBackoff(time.Second, 5*time.Minute)
	}
	return o
}

// ExponentialBackoff doubles base per failed attempt, capped at max.
func ExponentialBackoff(base, max time.Duration) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		d := base
		for i := 1; i < attempt; i++ {
			d *= 2
			if d >= max {
				return max
			}
		}
		return min(d, max)
	}
}

// permanentError marks a failure that retrying cannot fix.
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent wraps err so the consumer dead-letters the message without
// retrying. Permanent(nil) is nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// IsPermanent reports whether err (or anything it wraps) came from Permanent.
func IsPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}
