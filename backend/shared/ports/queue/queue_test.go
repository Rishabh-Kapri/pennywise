package queue

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExponentialBackoff(t *testing.T) {
	b := ExponentialBackoff(time.Second, 10*time.Second)
	assert.Equal(t, time.Second, b(1))
	assert.Equal(t, 2*time.Second, b(2))
	assert.Equal(t, 8*time.Second, b(4))
	assert.Equal(t, 10*time.Second, b(5))
	assert.Equal(t, 10*time.Second, b(1000))
}

func TestResolveConsumeOptionsClampsInvalid(t *testing.T) {
	o := ResolveConsumeOptions(WithConcurrency(0), WithMaxAttempts(-1), WithVisibilityTimeout(0), WithBackoff(nil))
	assert.Equal(t, 1, o.Concurrency)
	assert.Equal(t, 1, o.MaxAttempts)
	assert.Equal(t, 5*time.Minute, o.VisibilityTimeout)
	assert.NotNil(t, o.Backoff)
}

func TestWithHeadersMerges(t *testing.T) {
	o := ResolvePublishOptions(
		WithHeaders(map[string]string{"a": "1", "b": "1"}),
		WithHeaders(map[string]string{"b": "2"}),
	)
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, o.Headers)
}

func TestPermanent(t *testing.T) {
	base := errors.New("bad input")
	assert.Nil(t, Permanent(nil))
	assert.False(t, IsPermanent(base))

	wrapped := fmt.Errorf("handler: %w", Permanent(base))
	assert.True(t, IsPermanent(wrapped))
	assert.ErrorIs(t, wrapped, base)
}
