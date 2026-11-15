// ingest receives messages that might be bank transactions (email, SMS,
// direct API calls), turns them into one common shape and queues them for
// cipher. It never classifies anything itself.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/config"
	ingestFactory "github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/pubsub"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/worker"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

const (
	serviceName     = "ingest"
	shutdownTimeout = 10 * time.Second
)

func main() {
	cfg := config.Load() // first: it loads .env, which the logger reads too
	logger.Setup(serviceName)
	if err := run(cfg); err != nil {
		logger.Fatal("ingest stopped", "error", err)
	}
}

// run serves until SIGINT/SIGTERM or until a part fails, and returns once
// everything it started has stopped.
func run(cfg config.Config) error {
	// Cancelled on SIGINT/SIGTERM: everything started from it stops with it.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx = utils.WithInternalAuthToken(utils.WithLocalService(ctx, serviceName), cfg.InternalAuthToken)
	log := logger.Logger(ctx)

	// Repository ports; DATABASE_DRIVER picks the adapter behind them.
	database, err := factory.ConnectDB(ctx, factory.Config{Driver: cfg.DatabaseDriver, URL: cfg.DatabaseURL})
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer database.Close()

	var broker job.Broker           // nil: no queue, so ingestion answers 503
	var schedules job.ScheduleStore // nil: mailbox watches aren't renewed automatically
	jobs, err := factory.ConnectJob(ctx, factory.JobConfig{
		Driver:    cfg.JobDriver,
		BrokerURL: cfg.JobBrokerURL,
		StoreURL:  cfg.JobStoreURL,
		RedisURL:  cfg.RedisURL,
	})
	if err != nil {
		return fmt.Errorf("connect job queue: %w", err)
	}
	if jobs != nil {
		defer jobs.Close()
		broker = jobs.Broker
		schedules = jobs.Schedules
	}

	// Mail adapters, picked per connection by its Provider.
	mailProviders, err := ingestFactory.NewMailProviders(ctx, ingestFactory.MailConfig{
		Providers:      cfg.MailProviders,
		CredentialsKey: cfg.CredentialsKey,
		Gmail: ingestFactory.GmailConfig{
			WebClientID:     cfg.GoogleClientID,
			WebClientSecret: cfg.GoogleClientSecret,
			AndroidClientID: cfg.GoogleAndroidClientID,
			ProjectID:       cfg.GoogleProjectID,
			PubsubTopic:     cfg.GooglePubsubTopic,
		},
	})
	if err != nil {
		return fmt.Errorf("configure mail providers: %w", err)
	}

	serviceAuth, err := factory.NewServiceAuthenticator(ctx, factory.ServiceAuthConfig{
		Tokens:   cfg.ServiceTokens,
		OwnToken: cfg.InternalAuthToken,
	})
	if err != nil {
		return fmt.Errorf("invalid SERVICE_TOKENS: %w", err)
	}
	userAuth := sharedMiddleware.Authenticate(factory.NewAuthenticators(
		ctx,
		factory.AuthConfig{JWTSecret: cfg.JWTSecret},
		database.APIKey,
		database.Auth,
	)...)

	queue := service.NewEnqueuer(broker)
	ingestService := service.NewIngestService(queue)
	// One mailbox service: the push receiver queues mailbox.changed jobs
	// (Changed), the worker runs them (Sync).
	mailboxService := service.NewMailboxService(service.MailboxDeps{
		Queue:       queue,
		Schedules:   schedules,
		Connections: database.MailConnection,
		Budgets:     database.Budget,
		Providers:   mailProviders,
		Ingest:      ingestService,
	})

	// Gmail push receiver: each push becomes a mailbox.changed job.
	var subscriber *pubsub.GmailSubscriber
	if cfg.GooglePubsubSubscription == "" {
		log.Info("gmail push receiver off (SUB_NAME not set)")
	} else {
		if broker == nil {
			// Every push would be nacked and redelivered forever.
			return errors.New("SUB_NAME is set but no job queue is configured (JOB_DRIVER)")
		}
		subscriber = pubsub.NewGmailSubscriber(pubsub.Config{
			ProjectID:       cfg.GoogleProjectID,
			Subscription:    cfg.GooglePubsubSubscription,
			CredentialsJSON: cfg.GoogleCredentialsJSON,
		}, mailboxService)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(sharedMiddleware.StripInternalHeaders())
	router.Use(sharedMiddleware.RequestMetadata(serviceName))
	router.Use(sharedMiddleware.InternalRequestAuth(cfg.InternalAuthToken, serviceAuth))
	router.Use(sharedMiddleware.RequestLogger())
	api := router.Group("/api")
	api.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": serviceName, "status": "ok"})
	})
	// Callers are a user (API key or JWT) or a verified service, writing to
	// the budget in X-Budget-ID.
	handler.NewAPIHandler(ingestService, mailboxService).
		Register(api.Group("/ingest", userAuth, sharedMiddleware.BudgetIdMiddleware(database.Budget)))
	server := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	// The server, the job worker, the scheduler and the push receiver run
	// until ctx is cancelled or one of them fails, which stops the others.
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		log.Info("listening", "port", cfg.Port)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	group.Go(func() error {
		<-groupCtx.Done()
		// groupCtx is already cancelled, so shutdown gets its own deadline. It
		// keeps the values (logging) but not the cancellation.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(groupCtx), shutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	})
	group.Go(func() error {
		return worker.Run(groupCtx, worker.Deps{Broker: broker, Mailbox: mailboxService})
	})
	if schedules != nil {
		group.Go(func() error {
			jobrunner.RunScheduler(groupCtx, schedules, broker)
			return nil
		})
	}
	if subscriber != nil {
		group.Go(func() error { return subscriber.Run(groupCtx) })
	}

	// Waiting lets in-flight pushes finish enqueueing and running jobs finish
	// before the job queue and database close (the deferred Close calls).
	stopped := make(chan error, 1)
	go func() { stopped <- group.Wait() }()
	<-groupCtx.Done()
	log.Info("shutting down")
	select {
	case err := <-stopped:
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case <-time.After(shutdownTimeout):
		return errors.New("didn't stop in time")
	}
}
