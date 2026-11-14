// Package progress lets long-running service and adapter code report progress
// to whoever invoked it, without knowing who that is. A Temporal activity
// forwards reports to activity.RecordHeartbeat, a job handler to the job's
// progress; anywhere else reporting is a no-op.
package progress

import "context"

type contextKey struct{}

// Step is one progress report. Current and Total count work items (pages, say)
// and are zero when the step has no counter.
type Step struct {
	Name    string // short label, e.g. "predict:embed"
	Current int
	Total   int
}

// Func receives each report.
type Func func(Step)

// With returns a context whose Report calls invoke fn.
func With(ctx context.Context, fn Func) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, fn)
}

// Report invokes the installed reporter, if any.
func Report(ctx context.Context, step string) {
	ReportStep(ctx, Step{Name: step})
}

// ReportCount reports current of total items done for step.
func ReportCount(ctx context.Context, step string, current, total int) {
	ReportStep(ctx, Step{Name: step, Current: current, Total: total})
}

// ReportStep invokes the installed reporter, if any.
func ReportStep(ctx context.Context, s Step) {
	if fn, ok := ctx.Value(contextKey{}).(Func); ok {
		fn(s)
	}
}
