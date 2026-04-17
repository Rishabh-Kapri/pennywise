package transport

import (
	"context"
	"net/http"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
)

// Health check for dependent services before starting
func CheckHealth(ctx context.Context, name string, url string) {
	logger := logger.Logger(ctx)

	for i := 1; i <= 5; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			logger.Info("health check passed", "service", name)
			return
		}
		if err != nil {
			logger.Warn("health check failed, retrying...", "service", name, "attempt", i, "error", err)
		} else {
			resp.Body.Close()
			logger.Warn("health check failed, retrying...", "service", name, "attempt", i, "status", resp.StatusCode)
		}
		time.Sleep(2 * time.Second)
	}
	logger.Error("health check failed after retries", "service", name)
}
