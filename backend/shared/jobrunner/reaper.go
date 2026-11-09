package jobrunner

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

type reaper struct {
	id             string
	broker         job.Broker
	interval       time.Duration
	staleThreshold time.Duration
}

func newReaper(
	id string,
	broker job.Broker,
	interval time.Duration,
	staleThreshold time.Duration,
) *reaper {
	return &reaper{id, broker, interval, staleThreshold}
}

func (r *reaper) start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	log := logger.Logger(ctx)

	log.Info("reaper started", "interval_s", r.interval.Seconds(), "stale_threshold_s", r.staleThreshold.Seconds())

	for {
		select {
		case <-ctx.Done():
			log.Info("reaper stopping")
			return
		case <-ticker.C:
			n, err := r.broker.RequeueStaleRunning(ctx, r.staleThreshold, r.id)
			if err != nil {
				log.Warn("reaper requeue failed", "err", err)
				continue
			}
			if n > 0 {
				log.Info("reaper requeued stale jobs", "stale", n)
			}
		}
	}
}
