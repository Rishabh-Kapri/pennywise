// Package llmusage lets the prediction pipeline record what each LLM call
// cost — provider, model, tokens, duration — without the service layer knowing
// who is collecting. Activities install a collector for the email they are
// processing and hand the recorded calls back to the workflow, which stores
// them on the pipeline run. Outside a collector, recording is a no-op.
package llmusage

import (
	"context"
	"sync"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type contextKey struct{}

// Collector accumulates the calls made under one context. Safe for concurrent
// use: the fallback chains are sequential today, but nothing guarantees that.
type Collector struct {
	mu    sync.Mutex
	calls []sharedModel.LLMCall
}

// With returns a context carrying a fresh collector, plus the collector.
func With(ctx context.Context) (context.Context, *Collector) {
	collector := &Collector{}
	return context.WithValue(ctx, contextKey{}, collector), collector
}

// Record appends a call to the collector installed on ctx, if any.
func Record(ctx context.Context, call sharedModel.LLMCall) {
	collector, ok := ctx.Value(contextKey{}).(*Collector)
	if !ok || collector == nil {
		return
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.calls = append(collector.calls, call)
}

// Calls returns a copy of everything recorded so far.
func (c *Collector) Calls() []sharedModel.LLMCall {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.calls) == 0 {
		return nil
	}
	return append([]sharedModel.LLMCall(nil), c.calls...)
}
