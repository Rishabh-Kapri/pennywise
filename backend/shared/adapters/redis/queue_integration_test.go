package redis

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/queue"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests run against a real Redis (7+) when
// PENNYWISE_TEST_REDIS_URL is set, e.g.
//
//	docker run -d --name pw-test-redis -p 127.0.0.1:56379:6379 redis:7-alpine
//	PENNYWISE_TEST_REDIS_URL=redis://127.0.0.1:56379/0 go test ./adapters/redis/
//
// Each test uses its own key prefix and deletes its keys at the end.
const testRedisURLEnv = "PENNYWISE_TEST_REDIS_URL"

const testTopic = "jobs"

type harness struct {
	t      *testing.T
	client *goredis.Client
	q      queue.Queue
	keys   topicKeys
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	url := os.Getenv(testRedisURLEnv)
	if url == "" {
		t.Skipf("%s not set; skipping Redis integration test", testRedisURLEnv)
	}
	opts, err := goredis.ParseURL(url)
	require.NoError(t, err)
	client := goredis.NewClient(opts)
	require.NoError(t, client.Ping(context.Background()).Err())

	prefix := "pennywise-test:" + uuid.NewString()[:8]
	q := NewQueue(client, QueueOptions{Prefix: prefix, PollInterval: 50 * time.Millisecond})
	h := &harness{t: t, client: client, q: q, keys: q.(*redisQueue).keys(testTopic)}
	t.Cleanup(func() {
		client.Del(context.Background(), h.keys.stream, h.keys.delayed, h.keys.dead)
		client.Close()
	})
	return h
}

// consume runs Consume in the background and returns a func that stops it
// and waits for it to return.
func (h *harness) consume(handler queue.Handler, opts ...queue.ConsumeOption) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.q.Consume(ctx, testTopic, handler, opts...) }()
	return func() {
		cancel()
		select {
		case err := <-done:
			assert.NoError(h.t, err)
		case <-time.After(5 * time.Second):
			h.t.Fatal("Consume did not return after cancel")
		}
	}
}

func (h *harness) publish(body string, opts ...queue.PublishOption) string {
	id, err := h.q.Publish(context.Background(), testTopic, []byte(body), opts...)
	require.NoError(h.t, err)
	return id
}

func (h *harness) deadLetters() []envelope {
	msgs, err := h.client.XRange(context.Background(), h.keys.dead, "-", "+").Result()
	require.NoError(h.t, err)
	out := make([]envelope, 0, len(msgs))
	for _, m := range msgs {
		env, err := decode(m)
		require.NoError(h.t, err)
		out = append(out, env)
	}
	return out
}

// settled reports whether nothing is left anywhere but the dead-letter stream.
func (h *harness) settled() bool {
	ctx := context.Background()
	return h.client.XLen(ctx, h.keys.stream).Val() == 0 && h.client.ZCard(ctx, h.keys.delayed).Val() == 0
}

func fastRetries() []queue.ConsumeOption {
	return []queue.ConsumeOption{queue.WithBackoff(func(int) time.Duration { return 10 * time.Millisecond })}
}

func TestPublishConsumeAck(t *testing.T) {
	h := newHarness(t)
	id := h.publish("hello", queue.WithHeaders(map[string]string{"X-Correlation-ID": "c1"}))

	got := make(chan queue.Message, 1)
	stop := h.consume(func(_ context.Context, m queue.Message) error {
		got <- m
		return nil
	})
	defer stop()

	select {
	case m := <-got:
		assert.Equal(t, id, m.ID)
		assert.Equal(t, testTopic, m.Topic)
		assert.Equal(t, "hello", string(m.Body))
		assert.Equal(t, "c1", m.Headers["X-Correlation-ID"])
		assert.Equal(t, 1, m.Attempt)
		assert.WithinDuration(t, time.Now(), m.PublishedAt, 5*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("message not delivered")
	}
	assert.Eventually(t, h.settled, 2*time.Second, 20*time.Millisecond)
	assert.Empty(t, h.deadLetters())
}

func TestRetryKeepsIDAndCountsAttempts(t *testing.T) {
	h := newHarness(t)
	id := h.publish("flaky")

	var mu sync.Mutex
	var seen []queue.Message
	done := make(chan struct{})
	stop := h.consume(func(_ context.Context, m queue.Message) error {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, m)
		if m.Attempt < 3 {
			return errors.New("transient")
		}
		close(done)
		return nil
	}, fastRetries()...)
	defer stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("message never succeeded")
	}
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, 3)
	for i, m := range seen {
		assert.Equal(t, id, m.ID)
		assert.Equal(t, i+1, m.Attempt)
	}
	assert.Eventually(t, h.settled, 2*time.Second, 20*time.Millisecond)
}

func TestDeadLetterAfterMaxAttempts(t *testing.T) {
	h := newHarness(t)
	id := h.publish("doomed")

	var calls atomic.Int32
	stop := h.consume(func(context.Context, queue.Message) error {
		calls.Add(1)
		return errors.New("always fails")
	}, append(fastRetries(), queue.WithMaxAttempts(3))...)
	defer stop()

	require.Eventually(t, func() bool { return len(h.deadLetters()) == 1 }, 5*time.Second, 20*time.Millisecond)
	dead := h.deadLetters()[0]
	assert.Equal(t, id, dead.ID)
	assert.Equal(t, 3, dead.Attempt)
	assert.Equal(t, "always fails", dead.Error)
	assert.Equal(t, int32(3), calls.Load())
	assert.True(t, h.settled())
}

func TestPermanentErrorSkipsRetries(t *testing.T) {
	h := newHarness(t)
	h.publish("bad")

	var calls atomic.Int32
	stop := h.consume(func(context.Context, queue.Message) error {
		calls.Add(1)
		return queue.Permanent(errors.New("unparseable"))
	}, fastRetries()...)
	defer stop()

	require.Eventually(t, func() bool { return len(h.deadLetters()) == 1 }, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
	assert.Equal(t, 1, h.deadLetters()[0].Attempt)
}

func TestPanicIsRetried(t *testing.T) {
	h := newHarness(t)
	h.publish("boom")

	done := make(chan int, 1)
	stop := h.consume(func(_ context.Context, m queue.Message) error {
		if m.Attempt == 1 {
			panic("kaboom")
		}
		done <- m.Attempt
		return nil
	}, fastRetries()...)
	defer stop()

	select {
	case attempt := <-done:
		assert.Equal(t, 2, attempt)
	case <-time.After(5 * time.Second):
		t.Fatal("panicking message was not retried")
	}
}

func TestDelayedPublish(t *testing.T) {
	h := newHarness(t)
	start := time.Now()
	h.publish("later", queue.WithDelay(400*time.Millisecond))

	got := make(chan time.Time, 1)
	stop := h.consume(func(context.Context, queue.Message) error {
		got <- time.Now()
		return nil
	})
	defer stop()

	select {
	case at := <-got:
		assert.GreaterOrEqual(t, at.Sub(start), 400*time.Millisecond)
	case <-time.After(5 * time.Second):
		t.Fatal("delayed message not delivered")
	}
}

func TestVisibilityTimeoutFailsAttempt(t *testing.T) {
	h := newHarness(t)
	h.publish("slow")

	attempts := make(chan int, 2)
	stop := h.consume(func(ctx context.Context, m queue.Message) error {
		attempts <- m.Attempt
		if m.Attempt == 1 {
			<-ctx.Done() // hang until the visibility timeout cancels us
			return ctx.Err()
		}
		return nil
	}, append(fastRetries(), queue.WithVisibilityTimeout(200*time.Millisecond))...)
	defer stop()

	for want := 1; want <= 2; want++ {
		select {
		case got := <-attempts:
			assert.Equal(t, want, got)
		case <-time.After(5 * time.Second):
			t.Fatalf("attempt %d not delivered", want)
		}
	}
	assert.Eventually(t, h.settled, 2*time.Second, 20*time.Millisecond)
}

// A worker that read a message and died never settles it; another consumer
// must reclaim it after the visibility timeout and count that as an attempt.
func TestReclaimFromDeadConsumer(t *testing.T) {
	old := claimGrace
	claimGrace = 0
	t.Cleanup(func() { claimGrace = old })

	h := newHarness(t)
	ctx := context.Background()
	id := h.publish("orphan")
	require.NoError(t, h.q.(*redisQueue).ensureGroup(ctx, h.keys))
	_, err := h.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group: group, Consumer: "dead-worker", Streams: []string{h.keys.stream, ">"}, Count: 1,
	}).Result()
	require.NoError(t, err)

	got := make(chan queue.Message, 1)
	stop := h.consume(func(_ context.Context, m queue.Message) error {
		got <- m
		return nil
	}, append(fastRetries(), queue.WithVisibilityTimeout(200*time.Millisecond))...)
	defer stop()

	select {
	case m := <-got:
		assert.Equal(t, id, m.ID)
		assert.Equal(t, 2, m.Attempt)
	case <-time.After(5 * time.Second):
		t.Fatal("orphaned message was not reclaimed")
	}
	assert.Eventually(t, h.settled, 2*time.Second, 20*time.Millisecond)
}

func TestShutdownReleasesWithoutUsingAttempt(t *testing.T) {
	h := newHarness(t)
	id := h.publish("interrupted")

	started := make(chan struct{})
	stop := h.consume(func(ctx context.Context, _ queue.Message) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	<-started
	stop()

	// The release lands in the delayed set as due now; a promote racing the
	// shutdown may already have moved it to the ready stream. Either is fine.
	ctx := context.Background()
	payloads, err := h.client.ZRange(ctx, h.keys.delayed, 0, -1).Result()
	require.NoError(t, err)
	entries, err := h.client.XRange(ctx, h.keys.stream, "-", "+").Result()
	require.NoError(t, err)
	for _, e := range entries {
		payloads = append(payloads, e.Values[payloadField].(string))
	}
	require.Len(t, payloads, 1)
	var env envelope
	require.NoError(t, json.Unmarshal([]byte(payloads[0]), &env))
	assert.Equal(t, id, env.ID)
	assert.Equal(t, 1, env.Attempt)
	assert.Empty(t, h.deadLetters())
}

// Competing consumers: every message is handled, none by two consumers.
func TestCompetingConsumers(t *testing.T) {
	h := newHarness(t)
	const n = 50
	for i := range n {
		h.publish(string(rune('a' + i%26)))
	}

	var mu sync.Mutex
	seen := map[string]int{}
	handler := func(_ context.Context, m queue.Message) error {
		mu.Lock()
		seen[m.ID]++
		mu.Unlock()
		return nil
	}
	stop1 := h.consume(handler, queue.WithConcurrency(3))
	stop2 := h.consume(handler, queue.WithConcurrency(3))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == n
	}, 5*time.Second, 20*time.Millisecond)
	stop1()
	stop2()

	for id, count := range seen {
		assert.Equal(t, 1, count, "message %s handled %d times", id, count)
	}
	assert.True(t, h.settled())
}
