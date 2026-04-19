package transport

import (
	"context"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
)

// Health check for dependent services before starting with exponential backoff
func CheckHealth(ctx context.Context, name string, url string) {
	logger := logger.Logger(ctx)
	delay := 1.0 // initial delay in seconds

	for i := 1; i <= 5; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			logger.Info("health check passed", "service", name)
			return
		}
		if err != nil {
			logger.Warn("health check failed, retrying...", "service", name, "attempt", i, "delay", delay, "error", err)
			delay = 2 * delay
			// add jitter so that there is randomness to the retries, this is useful when there are multiple services using same backoff pattern
			delay = delay + (rand.Float64())
		} else {
			resp.Body.Close()
			logger.Warn("health check failed, retrying...", "service", name, "attempt", i, "delay", delay, "status", resp.StatusCode)
			delay = 2 * delay
			delay = delay + (rand.Float64())
		}
		time.Sleep(time.Duration(delay * float64(time.Second)))
	}
	logger.Error("health check failed after retries", "service", name)
}
