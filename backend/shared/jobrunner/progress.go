package jobrunner

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// progressInterval limits how often a counter is saved. A 2,000 email import
// shouldn't be 2,000 writes.
const progressInterval = time.Second

type progressKey struct{}

type progressReporter struct {
	broker job.Broker
	claim  job.Claim

	mu       sync.Mutex
	lastSent time.Time
	last     job.Progress
}

func withProgress(ctx context.Context, broker job.Broker, claim job.Claim) context.Context {
	return context.WithValue(ctx, progressKey{}, &progressReporter{broker: broker, claim: claim})
}

// ReportProgress saves what the running job is doing. It does nothing outside a
// job, and a failure is only logged: progress is never worth failing a job for.
// A new message always goes through; a counter is limited to once a second,
// except its last value.
func ReportProgress(ctx context.Context, progress job.Progress) {
	r, ok := ctx.Value(progressKey{}).(*progressReporter)
	if !ok {
		return
	}
	if !r.shouldSend(progress) {
		return
	}
	body, err := json.Marshal(progress)
	if err == nil {
		err = r.broker.UpdateProgress(ctx, r.claim, body)
	}
	if err != nil && !errs.HasCode(err, errs.CodeJobClaimLost) {
		logger.Logger(ctx).Warn("job progress not saved", "job_id", r.claim.JobID, "err", err)
	}
}

func (r *progressReporter) shouldSend(progress job.Progress) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	counterOnly := progress.Message == r.last.Message && progress.Current != progress.Total
	if counterOnly && time.Since(r.lastSent) < progressInterval {
		return false
	}
	r.last, r.lastSent = progress, time.Now()
	return true
}
