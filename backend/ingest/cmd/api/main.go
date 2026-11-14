// ingest receives messages that might be bank transactions (email, SMS,
// direct API calls), turns them into one common shape and queues them for
// cipher. It never classifies anything itself.
package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/pubsub"

	ingestFactory "github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/handler"

	// "github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/pubsub"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/worker"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

const (
	serviceName     = "ingest"
	shutdownTimeout = 10 * time.Second
)

func main() {
	logger.Setup(serviceName)
	cfg := config.Load()

	// Cancelled on SIGINT/SIGTERM: everything started from it stops with it.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx = utils.WithInternalAuthToken(utils.WithServiceName(ctx, serviceName), cfg.InternalAuthToken)
	log := logger.Logger(ctx)

	// Repository ports; DATABASE_DRIVER picks the adapter behind them.
	database, err := factory.ConnectDB(ctx, factory.Config{Driver: cfg.DatabaseDriver, URL: cfg.DatabaseURL})
	if err != nil {
		logger.Fatal("connect database", "error", err)
	}
	defer database.Close()

	var broker job.Broker           // nil: no queue, so ingestion answers 503
	var schedules job.ScheduleStore // nil: mailbox watches aren't renewed automatically
	// Job queue for accepted messages. nil when JOB_DRIVER is unset.
	jobs, err := factory.ConnectJob(ctx, factory.JobConfig{
		Driver:    cfg.JobDriver,
		BrokerURL: cfg.JobBrokerURL,
		StoreURL:  cfg.JobStoreURL,
		RedisURL:  cfg.RedisURL,
	})
	if err != nil {
		logger.Fatal("connect job queue", "error", err)
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
		logger.Fatal("configure mail providers", "error", err)
	}

	serviceAuth, err := factory.NewServiceAuthenticator(ctx, factory.ServiceAuthConfig{
		Tokens:   cfg.ServiceTokens,
		OwnToken: cfg.InternalAuthToken,
	})
	if err != nil {
		logger.Fatal("invalid SERVICE_TOKENS", "error", err)
	}
	userAuth := sharedMiddleware.Authenticate(factory.NewAuthenticators(
		ctx,
		factory.AuthConfig{JWTSecret: cfg.JWTSecret},
		database.APIKey,
		database.Auth,
	)...)

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

	ingestService := service.NewIngestService(broker)
	// One mailbox service: the push receiver queues mailbox.changed jobs
	// (Changed), the worker runs them (Sync).
	mailboxService := service.NewMailboxService(broker, service.MailboxDeps{
		Schedules:   schedules,
		Connections: database.MailConnection,
		Budgets:     database.Budget,
		Providers:   mailProviders,
		Ingest:      ingestService,
	})
	apiHandler := handler.NewAPIHandler(ingestService, mailboxService)

	// Gmail push receiver: each push becomes a mailbox.changed job.
	subscriberDone := make(chan struct{})
	if cfg.GooglePubsubSubscription == "" {
		log.Info("gmail push receiver off (SUB_NAME not set)")
		close(subscriberDone)
	} else {
		if broker == nil {
			// Every push would be nacked and redelivered forever.
			logger.Fatal("SUB_NAME is set but no job queue is configured (JOB_DRIVER)")
		}
		subscriber := pubsub.NewGmailSubscriber(pubsub.Config{
			ProjectID:       cfg.GoogleProjectID,
			Subscription:    cfg.GooglePubsubSubscription,
			CredentialsJSON: cfg.GoogleCredentialsJSON,
		}, mailboxService)
		go func() {
			defer close(subscriberDone)
			if err := subscriber.Run(ctx); err != nil && ctx.Err() == nil {
				logger.Fatal("gmail push receiver stopped", "error", err)
			}
		}()
	}

	// Callers are a user (API key or JWT) or a verified service, writing to
	// the budget in X-Budget-ID.
	ingest := api.Group("/ingest")
	ingest.Use(userAuth, sharedMiddleware.BudgetIdMiddleware(database.BudgetMember))
	ingest.POST("/messages", sharedMiddleware.RequireScopes(model.ScopeWrite), apiHandler.Ingest)
	ingest.POST("/sms", sharedMiddleware.RequireScopes(model.ScopeWrite), apiHandler.Sms)
	ingest.POST("/connections", sharedMiddleware.RequireScopes(model.ScopeWrite), apiHandler.Connect)
	ingest.GET("/connections", sharedMiddleware.RequireScopes(model.ScopeRead), apiHandler.ListConnections)
	ingest.PATCH("/connections/:id", sharedMiddleware.RequireScopes(model.ScopeWrite), apiHandler.UpdateConnection)
	ingest.DELETE("/connections/:id", sharedMiddleware.RequireScopes(model.ScopeWrite), apiHandler.Disconnect)
	ingest.GET(
		"/connections/:id/backfill",
		sharedMiddleware.RequireScopes(model.ScopeRead),
		apiHandler.BackfillCandidates,
	)
	ingest.POST("/connections/:id/backfill", sharedMiddleware.RequireScopes(model.ScopeWrite), apiHandler.Backfill)
	ingest.POST("/test", sharedMiddleware.RequireScopes(model.ScopeWrite), apiHandler.Test)

	serverErr := make(chan error, 1)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: router}
	go func() {
		log.Info("listening", "port", cfg.Port)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
	}()

	if schedules != nil {
		go jobrunner.RunScheduler(ctx, schedules, broker)
	}

	workerError := make(chan error, 1)
	workerDone := make(chan struct{})
	// Job worker: runs mailbox.changed jobs. Off without a job queue.
	go func() {
		defer close(workerDone)
		if err := worker.Setup(ctx, worker.Deps{
			Queues:  cfg.JobQueues,
			Broker:  broker,
			Mailbox: mailboxService,
		}); err != nil {
			workerError <- err
		}
	}()

	// something happened with the server or worker or initial ctx cancelled
	select {
	case <-ctx.Done():
		log.Info("server shutdown")
	case e := <-serverErr:
		if e != nil {
			log.Error("error starting server", "err", e)
		}
	case e := <-workerError:
		if e != nil && !errors.Is(e, context.Canceled) {
			log.Error("unexpected worker error", "err", e)
		}
	}
	stop()

	// ctx is already cancelled, so shutdown gets its own deadline. It keeps
	// ctx's values (logging) but not its cancellation.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", "error", err)
	}
	// Let in-flight pushes finish enqueueing and running jobs finish before
	// the job queue and database close (the deferred Close calls).
	select {
	case <-subscriberDone:
		log.Info("subscriber done")
	case <-shutdownCtx.Done():
		log.Error("gmail push receiver didn't stop in time")
	}
	select {
	case <-workerDone:
		log.Info("worker done")
	case <-shutdownCtx.Done():
		log.Error("job worker didn't stop in time")
	}
}
