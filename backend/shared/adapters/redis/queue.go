// Package redis is the Redis adapter for the queue port (shared/ports/queue),
// built on Redis Streams and consumer groups.
//
// Per topic, three keys share a hash tag so the Lua scripts stay valid on
// Redis Cluster:
//
//	<prefix>:{<topic>}          stream: messages ready for delivery
//	<prefix>:{<topic>}:delayed  sorted set: delayed publishes and retries,
//	                            scored by the unix-ms time they become ready
//	<prefix>:{<topic>}:dead     stream: dead-lettered messages (capped)
//
// Every topic has one consumer group, so a topic is a work queue. Settled
// entries are XDEL'd, so the ready stream holds only undelivered and
// in-flight work. Settling (ack, retry, dead-letter) is one Lua script that
// first checks this consumer still owns the entry, so a slow handler and the
// reaper that reclaimed its message cannot both settle it.
package redis

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/queue"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const (
	// group is the single consumer group on every topic stream.
	group = "workers"
	// payloadField is the stream field holding the encoded envelope.
	payloadField = "m"

	defaultPrefix           = "pennywise:queue"
	defaultPollInterval     = time.Second
	defaultDeadLetterMaxLen = 10000

	// settleTimeout bounds the Redis call that settles a message. It runs on
	// a context detached from the consumer's, so shutdown can still settle.
	settleTimeout = 5 * time.Second
	// batchSize caps the entries promoted or reclaimed per Redis call.
	batchSize = 100
)

var (
	// claimGrace is added to the visibility timeout before the reaper
	// reclaims an entry, so the handler's own timeout settles it first. A var
	// only so tests can shorten it.
	claimGrace = 30 * time.Second

	//go:embed scripts/promote.lua
	promoteSource string
	//go:embed scripts/settle.lua
	settleSource string

	promoteScript = goredis.NewScript(promoteSource)
	settleScript  = goredis.NewScript(settleSource)

	errVisibilityTimeout = errors.New("queue: visibility timeout exceeded")
)

// QueueOptions tunes the adapter. Zero values take the defaults.
type QueueOptions struct {
	// Prefix namespaces every key. Default "pennywise:queue".
	Prefix string
	// PollInterval is how long a worker blocks waiting for a message, and
	// how often due delayed messages are promoted and stuck ones reclaimed.
	// It is also the worst-case lateness of a delayed message. Default 1s.
	PollInterval time.Duration
	// DeadLetterMaxLen approximately caps each topic's dead-letter stream.
	// Default 10000.
	DeadLetterMaxLen int64
}

type redisQueue struct {
	client goredis.UniversalClient
	opts   QueueOptions
}

// NewQueue returns the queue port backed by client. The caller keeps
// ownership of client and closes it after every Consume has returned.
func NewQueue(client goredis.UniversalClient, opts QueueOptions) queue.Queue {
	if opts.Prefix == "" {
		opts.Prefix = defaultPrefix
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultPollInterval
	}
	if opts.DeadLetterMaxLen <= 0 {
		opts.DeadLetterMaxLen = defaultDeadLetterMaxLen
	}
	return &redisQueue{client: client, opts: opts}
}

// envelope is what is stored in the stream and the delayed set. The JSON
// form is the whole message, so moving it between keys is a string copy.
type envelope struct {
	ID          string            `json:"id"`
	Body        []byte            `json:"body"`
	Headers     map[string]string `json:"headers,omitempty"`
	Attempt     int               `json:"attempt"`
	PublishedAt int64             `json:"publishedAt"`

	// Set only on dead-lettered messages.
	Error    string `json:"error,omitempty"`
	FailedAt int64  `json:"failedAt,omitempty"`
}

type topicKeys struct {
	stream, delayed, dead string
}

func (q *redisQueue) keys(topic string) topicKeys {
	base := fmt.Sprintf("%s:{%s}", q.opts.Prefix, topic)
	return topicKeys{stream: base, delayed: base + ":delayed", dead: base + ":dead"}
}

func (q *redisQueue) Publish(ctx context.Context, topic string, body []byte, opts ...queue.PublishOption) (string, error) {
	if topic == "" {
		return "", errors.New("queue: empty topic")
	}
	o := queue.ResolvePublishOptions(opts...)
	now := time.Now()
	env := envelope{
		ID:          uuid.NewString(),
		Body:        body,
		Headers:     o.Headers,
		Attempt:     1,
		PublishedAt: now.UnixMilli(),
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("queue: encode message: %w", err)
	}

	k := q.keys(topic)
	if o.Delay > 0 {
		err = q.client.ZAdd(ctx, k.delayed, goredis.Z{
			Score:  float64(now.Add(o.Delay).UnixMilli()),
			Member: payload,
		}).Err()
	} else {
		err = q.client.XAdd(ctx, &goredis.XAddArgs{
			Stream: k.stream,
			Values: []any{payloadField, payload},
		}).Err()
	}
	if err != nil {
		return "", fmt.Errorf("queue: publish to %s: %w", topic, err)
	}
	return env.ID, nil
}

func (q *redisQueue) Consume(ctx context.Context, topic string, handler queue.Handler, opts ...queue.ConsumeOption) error {
	if topic == "" {
		return errors.New("queue: empty topic")
	}
	k := q.keys(topic)
	if err := q.ensureGroup(ctx, k); err != nil {
		return err
	}

	c := &consumer{
		q:       q,
		topic:   topic,
		keys:    k,
		name:    consumerName(),
		handler: handler,
		opts:    queue.ResolveConsumeOptions(opts...),
	}
	logger.Logger(ctx).Info("queue consumer started",
		"topic", topic, "consumer", c.name, "concurrency", c.opts.Concurrency)

	var wg sync.WaitGroup
	wg.Go(func() { c.maintain(ctx) })
	for range c.opts.Concurrency {
		wg.Go(func() { c.work(ctx) })
	}
	wg.Wait()

	logger.Logger(ctx).Info("queue consumer stopped", "topic", topic, "consumer", c.name)
	return nil
}

// ensureGroup creates the topic's stream and consumer group if missing. The
// group starts at ID 0 so messages published before any consumer existed
// are delivered.
func (q *redisQueue) ensureGroup(ctx context.Context, k topicKeys) error {
	err := q.client.XGroupCreateMkStream(ctx, k.stream, group, "0").Err()
	if err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("queue: create consumer group on %s: %w", k.stream, err)
	}
	return nil
}

// consumer is one Consume call: a set of workers plus a maintenance loop,
// all reading as the same Redis consumer name.
type consumer struct {
	q       *redisQueue
	topic   string
	keys    topicKeys
	name    string
	handler queue.Handler
	opts    queue.ConsumeOptions
}

// consumerName is unique per Consume call, so a restarted process never
// inherits a dead one's pending entries; the reaper reclaims those.
func consumerName() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.NewString()[:8])
}

// work reads and handles messages one at a time until ctx is cancelled.
func (c *consumer) work(ctx context.Context) {
	for ctx.Err() == nil {
		streams, err := c.q.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    group,
			Consumer: c.name,
			Streams:  []string{c.keys.stream, ">"},
			Count:    1,
			Block:    c.q.opts.PollInterval,
		}).Result()
		if errors.Is(err, goredis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.pause(ctx, "read", err)
			continue
		}
		for _, s := range streams {
			for _, m := range s.Messages {
				c.handle(ctx, m)
			}
		}
	}
}

// pause logs a Redis failure, recreates the group if the stream was
// deleted under us, and pauses before the caller retries.
func (c *consumer) pause(ctx context.Context, op string, err error) {
	logger.Logger(ctx).Error("queue redis call failed", "topic", c.topic, "op", op, "error", err)
	if strings.HasPrefix(err.Error(), "NOGROUP") {
		if gerr := c.q.ensureGroup(ctx, c.keys); gerr != nil {
			logger.Logger(ctx).Error("queue recreate group failed", "topic", c.topic, "error", gerr)
		}
	}
	sleep(ctx, c.q.opts.PollInterval)
}

func (c *consumer) handle(ctx context.Context, m goredis.XMessage) {
	env, err := decode(m)
	if err != nil {
		// Unreadable: no handler can fix it. Keep the raw payload for a human.
		raw, _ := m.Values[payloadField].(string)
		c.deadLetter(ctx, m.ID, envelope{Body: []byte(raw)}, queue.Permanent(err))
		return
	}

	hctx, cancel := context.WithTimeout(ctx, c.opts.VisibilityTimeout)
	herr := c.run(hctx, env)
	timedOut := errors.Is(hctx.Err(), context.DeadlineExceeded)
	cancel()

	switch {
	case herr == nil:
		c.settle(ctx, m.ID, "ack", nil, 0)
	case ctx.Err() != nil:
		// Shutdown interrupted the handler: put it back as the same attempt.
		c.retry(ctx, m.ID, env, 0)
	default:
		if timedOut && !errors.Is(herr, errVisibilityTimeout) {
			herr = fmt.Errorf("%w: %w", errVisibilityTimeout, herr)
		}
		c.fail(ctx, m.ID, env, herr)
	}
}

// run calls the handler, turning a panic into an error.
func (c *consumer) run(ctx context.Context, env envelope) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("queue: handler panic: %v", r)
		}
	}()
	headers := env.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	return c.handler(ctx, queue.Message{
		ID:          env.ID,
		Topic:       c.topic,
		Body:        env.Body,
		Headers:     headers,
		Attempt:     env.Attempt,
		PublishedAt: time.UnixMilli(env.PublishedAt),
	})
}

// fail retries env after the backoff, or dead-letters it when the error is
// permanent or the attempts are used up.
func (c *consumer) fail(ctx context.Context, entryID string, env envelope, cause error) {
	if queue.IsPermanent(cause) || env.Attempt >= c.opts.MaxAttempts {
		c.deadLetter(ctx, entryID, env, cause)
		return
	}
	logger.Logger(ctx).Warn("queue message failed; retrying",
		"topic", c.topic, "messageId", env.ID, "attempt", env.Attempt, "error", cause)
	delay := c.opts.Backoff(env.Attempt)
	env.Attempt++
	c.retry(ctx, entryID, env, delay)
}

func (c *consumer) retry(ctx context.Context, entryID string, env envelope, delay time.Duration) {
	payload, err := json.Marshal(env)
	if err != nil {
		c.deadLetter(ctx, entryID, env, queue.Permanent(err))
		return
	}
	c.settle(ctx, entryID, "retry", payload, time.Now().Add(delay).UnixMilli())
}

func (c *consumer) deadLetter(ctx context.Context, entryID string, env envelope, cause error) {
	logger.Logger(ctx).Error("queue message dead-lettered",
		"topic", c.topic, "messageId", env.ID, "attempt", env.Attempt, "error", cause)
	env.Error = cause.Error()
	env.FailedAt = time.Now().UnixMilli()
	payload, _ := json.Marshal(env) // only []byte, strings and ints: cannot fail
	c.settle(ctx, entryID, "dead", payload, 0)
}

// settle runs the settle script: ack the entry and, for retry/dead, move the
// payload to the delayed set or dead-letter stream, all only if this
// consumer still owns the entry. If it does not, the reaper reclaimed it and
// has settled (or will settle) it instead.
func (c *consumer) settle(ctx context.Context, entryID, action string, payload []byte, score int64) {
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	owned, err := settleScript.Run(sctx, c.q.client,
		[]string{c.keys.stream, c.keys.delayed, c.keys.dead},
		group, c.name, entryID, action, payload, score, c.q.opts.DeadLetterMaxLen,
	).Int()
	if err != nil {
		// The entry stays pending; the reaper redelivers it after the
		// visibility timeout.
		logger.Logger(ctx).Error("queue settle failed",
			"topic", c.topic, "entryId", entryID, "action", action, "error", err)
		return
	}
	if owned == 0 {
		logger.Logger(ctx).Warn("queue entry reclaimed before settle; dropping result",
			"topic", c.topic, "entryId", entryID, "action", action)
	}
}

// maintain promotes due delayed messages and reclaims entries whose handler
// outlived the visibility timeout (crashed or hung worker), every poll
// interval until ctx is cancelled.
func (c *consumer) maintain(ctx context.Context) {
	t := time.NewTicker(c.q.opts.PollInterval)
	defer t.Stop()
	for {
		c.promote(ctx)
		c.reclaim(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (c *consumer) promote(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := promoteScript.Run(ctx, c.q.client,
			[]string{c.keys.stream, c.keys.delayed},
			time.Now().UnixMilli(), batchSize,
		).Int()
		if err != nil {
			if ctx.Err() == nil {
				c.pause(ctx, "promote", err)
			}
			return
		}
		if n < batchSize {
			return
		}
	}
}

// reclaim takes ownership of stuck entries and fails them as one attempt, so
// a message that kills or hangs every worker is eventually dead-lettered.
func (c *consumer) reclaim(ctx context.Context) {
	start := "0-0"
	for ctx.Err() == nil {
		msgs, next, err := c.q.client.XAutoClaim(ctx, &goredis.XAutoClaimArgs{
			Stream:   c.keys.stream,
			Group:    group,
			Consumer: c.name,
			MinIdle:  c.opts.VisibilityTimeout + claimGrace,
			Start:    start,
			Count:    batchSize,
		}).Result()
		if err != nil {
			if ctx.Err() == nil {
				c.pause(ctx, "reclaim", err)
			}
			return
		}
		for _, m := range msgs {
			env, derr := decode(m)
			if derr != nil {
				raw, _ := m.Values[payloadField].(string)
				c.deadLetter(ctx, m.ID, envelope{Body: []byte(raw)}, queue.Permanent(derr))
				continue
			}
			c.fail(ctx, m.ID, env, errVisibilityTimeout)
		}
		if next == "0-0" || next == "" {
			return
		}
		start = next
	}
}

func decode(m goredis.XMessage) (envelope, error) {
	raw, ok := m.Values[payloadField].(string)
	if !ok {
		return envelope{}, fmt.Errorf("queue: entry %s has no %q field", m.ID, payloadField)
	}
	var env envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return envelope{}, fmt.Errorf("queue: decode entry %s: %w", m.ID, err)
	}
	if env.Attempt < 1 {
		env.Attempt = 1
	}
	return env, nil
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
