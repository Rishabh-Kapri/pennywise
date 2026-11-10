package factory

import (
	"context"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/memory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/postgres"
	redisAdapter "github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/redis"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	goredis "github.com/redis/go-redis/v9"
)

const (
	JobDriverPostgres = "postgres"
	JobDriverNone     = "none"
	JobDriverMemory   = "memory"
)

type JobConfig struct {
	Driver    string
	BrokerURL string
	StoreURL  string
	// RedisURL carries job updates between services. Empty: none are sent.
	RedisURL string
}

func (c JobConfig) driver() string {
	d := strings.ToLower(strings.TrimSpace(c.Driver))
	if d == "" {
		return "none"
	}
	return d
}

type JobConn struct {
	// Broker is the configured broker adapter. When a job store is
	// configured it is wrapped in job.ObservedBroker, which records every
	// state change in the store's job history.
	Broker job.Broker
	Store  job.JobStore
	// Schedules keeps recurring jobs. Nil on the memory driver.
	Schedules job.ScheduleStore
	// Updates delivers every service's job changes. Nil without a RedisURL.
	Updates job.UpdateSource
	close   func()
}

func (j *JobConn) Close() {
	if j == nil {
		return
	}
	if j.close != nil {
		j.close()
	}
}

// connectJobUpdates is best effort: live updates are a convenience, so a bad
// URL shouldn't stop the service from running jobs.
func connectJobUpdates(ctx context.Context, url string) (*redisAdapter.JobUpdates, func()) {
	if url == "" {
		logger.Logger(ctx).Warn("job updates off (REDIS_URL not set)")
		return nil, func() {}
	}
	options, err := goredis.ParseURL(url)
	if err != nil {
		logger.Logger(ctx).Error("job updates off: invalid redis url", "error", err)
		return nil, func() {}
	}
	client := goredis.NewClient(options)
	return redisAdapter.NewJobUpdates(client), func() { _ = client.Close() }
}

func ConnectJob(ctx context.Context, cfg JobConfig) (*JobConn, error) {
	driver := cfg.driver()
	switch driver {
	case JobDriverPostgres:
		if cfg.BrokerURL == "" {
			return nil, errs.New(errs.CodeJobDatabaseURLMissing, "no database url provided for adapter: %q", driver)
		}
		if cfg.StoreURL == "" {
			return nil, errs.New(errs.CodeJobStoreURLMissing, "no database url provided for job store")
		}
		brokerPool, closeBroker, err := acquirePool(ctx, cfg.BrokerURL)
		if err != nil {
			return nil, err
		}
		// The store and its history live at StoreURL. acquirePool hands back
		// the same pool when both URLs match.
		storePool, closeStore, err := acquirePool(ctx, cfg.StoreURL)
		if err != nil {
			closeBroker()
			return nil, err
		}
		broker := job.NewObservedBroker(
			postgres.NewPgJobBrokerAdapter(brokerPool),
			postgres.NewPgJobLogAdapter(storePool),
		)
		store := postgres.NewPgJobStoreAdapter(storePool)
		logger.Logger(ctx).Info("Broker adapter configured", "driver", driver)
		updates, closeUpdates := connectJobUpdates(ctx, cfg.RedisURL)
		conn := &JobConn{
			Broker:    broker,
			Store:     store,
			Schedules: postgres.NewPgJobScheduleAdapter(storePool),
			close: func() {
				closeUpdates()
				closeStore()
				closeBroker()
			},
		}
		if updates != nil {
			broker.WithNotifier(updates)
			conn.Updates = updates
		}
		return conn, nil
	case JobDriverMemory:
		// Local testing: the job history is printed to the console, not stored.
		broker := job.NewObservedBroker(
			memory.NewMemoryJobBrokerAdapter(),
			memory.NewMemoryJobLogAdapter(),
		)
		logger.Logger(ctx).Info("Broker adapter configured", "driver", driver)
		return &JobConn{
			Broker: broker,
			Store:  nil,
			close:  nil,
		}, nil
	case JobDriverNone:
		// no job driver configured
		logger.Logger(ctx).Warn("no job driver configured")
		return nil, nil
	default:
		return nil, errs.New(
			errs.CodeAdapterNotConfigured,
			"no adapter configured for job with name: %q",
			driver,
		)
	}
}
