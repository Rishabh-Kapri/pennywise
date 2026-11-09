package factory

import (
	"context"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/memory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/postgres"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
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
	close     func()
}

func (j *JobConn) Close() {
	if j == nil {
		return
	}
	if j.close != nil {
		j.close()
	}
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
		return &JobConn{
			Broker:    broker,
			Store:     store,
			Schedules: postgres.NewPgJobScheduleAdapter(storePool),
			close: func() {
				closeStore()
				closeBroker()
			},
		}, nil
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
