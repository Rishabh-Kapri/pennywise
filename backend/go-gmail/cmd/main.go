package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/gmail"
	temporalActivities "github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/temporal"

	// "github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/pubsub"

	// "github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/pubsub"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	sharedTemporal "github.com/Rishabh-Kapri/pennywise/backend/shared/temporal"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"

	tc "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// package-level Temporal client, set in main() and used by handlers
var temporalClient tc.Client

/*
* Main entry point for running the gmail api server
* Simple http handler apis
 */

func healthPage(c *gin.Context) {
	log := logger.Logger(c.Request.Context())
	log.Info("health check passed")
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

func temporalHandler(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.Logger(ctx)

	var reqData gmail.EventData
	if err := c.ShouldBindJSON(&reqData); err != nil {
		log.Error("error unmarshalling request body", "error", err)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error unmarshalling request body", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": wrappedErr.Error()})
		return
	}

	we, err := temporalClient.ExecuteWorkflow(
		ctx,
		tc.StartWorkflowOptions{
			TaskQueue: sharedModel.PennywiseTaskQueue,
		},
		sharedModel.EmailToTransactionWorkflowName,
		sharedModel.EmailWorflowInput{
			Email:     reqData.Email,
			HistoryId: reqData.HistoryId,
		},
	)
	if err != nil {
		log.Error("error starting workflow", "error", err)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error starting workflow", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": wrappedErr.Error()})
		return
	}

	log.Info("workflow started", "workflowId", we.GetID(), "runId", we.GetRunID())
	c.JSON(http.StatusOK, gin.H{"workflowId": we.GetID(), "runId": we.GetRunID()})
}

func watchHandler(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.Logger(ctx)

	var reqData gmail.GmailSyncRequest
	if err := c.ShouldBindJSON(&reqData); err != nil {
		log.Error("error unmarshalling request body", "error", err)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error unmarshalling request body", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": wrappedErr.Error()})
		return
	}

	historyId, expiration, err := gmail.NewService().WatchHandler(ctx, reqData)
	if err != nil {
		log.Error("error in watch handler", "error", err)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error in watch handler", err)
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": wrappedErr.Error()},
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{"historyID": historyId, "expiration": expiration})
}

func setupWatchTestHandler(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.Logger(ctx)

	var reqData gmail.GmailSyncRequest
	if err := c.ShouldBindJSON(&reqData); err != nil {
		log.Error("error unmarshalling request body", "error", err)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error unmarshalling request body", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": wrappedErr.Error()})
		return
	}

	historyId, expiration, err := gmail.NewService().WatchHandler(ctx, reqData)
	if err != nil {
		log.Error("error setting up gmail watch", "error", err)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error setting up gmail watch", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": wrappedErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"historyID": historyId, "expiration": expiration})
}

func connectToTemporal(ctx context.Context, cfg config.Config) (tc.Client, error) {
	logger.Logger(ctx).Info("temporal", "host", cfg.TemporalServerHost, "port", cfg.TemporalServerPort)
	c, err := tc.Dial(tc.Options{
		HostPort:           cfg.TemporalServerHost + ":" + cfg.TemporalServerPort,
		Logger:             logger.Logger(ctx),
		ContextPropagators: sharedTemporal.ContextPropagators(),
	})
	if err != nil {
		return nil, err
	}
	logger.Logger(ctx).Info("connected to temporal")
	return c, nil
}

func main() {
	logger.Setup("gmail-pubsub")
	cfg := config.LoadConfig()
	ctx := utils.WithInternalAuthToken(
		utils.WithServiceName(context.Background(), "gmail-pubsub"),
		cfg.InternalAuthToken,
	)
	log := logger.Logger(ctx)

	transport.CheckHealth(ctx, "pennywise-api", cfg.PennywiseServiceURL+"/api")
	transport.CheckHealth(ctx, "cipher", cfg.CipherServiceURL+"/api")
	// transport.CheckHealth(ctx, "mlp-api", cfg.MLPServiceURL+"/health")

	dbConn, err := db.ConnectWithURL(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	log.Info("db connected", "url", cfg.DatabaseURL)

	if cfg.TemporalServerHost != "" {
		temporalClient, err = connectToTemporal(ctx, *cfg)
		if err != nil {
			logger.Fatal("Unable to connect to Temporal", "error", err)
		}
		defer temporalClient.Close()

		w := worker.New(temporalClient, sharedModel.GmailActivitiesTaskQueue, worker.Options{
			UseBuildIDForVersioning: false,
			BackgroundActivityContext: utils.WithInternalAuthToken(
				utils.WithServiceName(context.Background(), "gmail-pubsub"),
				cfg.InternalAuthToken,
			),
		})
		w.RegisterActivity(&temporalActivities.WatchGmailActivity{Gmail: gmail.NewService()})
		go func() {
			if err := w.Run(worker.InterruptCh()); err != nil {
				logger.Fatal("Temporal activity worker failed", "error", err)
			}
		}()
	}

	// pennywiseTransport := httpclient.NewHttpTransport(cfg.PennywiseServiceURL)
	// pennywiseClient := transport.NewClient("pennywise-api", pennywiseTransport)

	// go func() {
	// 	if err := w.Run(worker.InterruptCh()); err != nil {
	// 		logger.Fatal("Temporal activity worker failed", "error", err)
	// 	}
	// }()

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(sharedMiddleware.StripInternalHeaders())
	router.Use(sharedMiddleware.RequestMetadata("gmail-pubsub"))
	router.Use(sharedMiddleware.InternalRequestAuth(cfg.InternalAuthToken))
	router.Use(sharedMiddleware.RequestLogger())

	api := router.Group("/api")
	api.GET("", healthPage)

	budgetRepo := db.NewBudgetRepository(dbConn)
	budgetApiGroup := api.Group("")
	budgetApiGroup.Use(sharedMiddleware.BudgetIdMiddleware(budgetRepo))

	budgetApiGroup.POST("/watch", watchHandler)
	budgetApiGroup.POST("/test/setup-watch", setupWatchTestHandler)
	budgetApiGroup.POST("/temporal", temporalHandler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// go pubsub.PullMessages(ctx)

	go func() {
		log.Info("Server listening on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			logger.Fatal("server error", "error", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := server.Shutdown(context.Background()); err != nil {
		logger.Fatal("server forced to shutdown", "error", err)
	}
}
