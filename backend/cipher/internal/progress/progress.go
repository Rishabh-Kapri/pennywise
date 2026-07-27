// Package progress lets long-running service code report step-level progress
// to whoever invoked it, without knowing about Temporal. Activities install a
// reporter that forwards to activity.RecordHeartbeat; outside activities
// reporting is a no-op.
package progress

import "context"

type contextKey struct{}

// Func receives a short step label, e.g. "predict:embed".
type Func func(step string)

// With returns a context whose Report calls invoke fn.
func With(ctx context.Context, fn Func) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, fn)
}

// Report invokes the installed reporter, if any.
func Report(ctx context.Context, step string) {
	if fn, ok := ctx.Value(contextKey{}).(Func); ok {
		fn(step)
	}
}
