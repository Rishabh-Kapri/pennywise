package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	apiClient "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/storage"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/websocket"
	jobWorker "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/worker"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// workerShutdownTimeout is a little over the job worker's own 30s, so it can
// report its result first.
const workerShutdownTimeout = 35 * time.Second

var allowedOrigins = []string{
	"http://localhost:5000",
	"http://localhost:5173",
	"http://localhost:5100",
	"https://pennywise-fe-production.up.railway.app",
	"https://react-fe-production-8fe5.up.railway.app",
	"https://dev.pennywise.cloud",
	"https://pennywise.cloud",
	"https://www.pennywise.cloud",
	"https://pennywise.nastydomain.space",
	"https://react-fe-dev.up.railway.app",
}

// serviceClient calls another Pennywise service as this one: every request
// carries this service's own internal token.
func serviceClient(name, url, ownToken string) *transport.Client {
	headers := map[string][]string{}
	if ownToken != "" {
		headers[utils.HeaderInternalToken] = []string{ownToken}
	}
	return transport.NewClient(name, httpclient.NewHttpTransport(url), transport.WithDefaultHeaders(headers))
}

func main() {
	config := config.Load()
	logger.Setup(config.ServiceName)
	ctx := utils.WithInternalAuthToken(
		utils.WithLocalService(context.Background(), config.ServiceName),
		config.InternalAuthToken,
	)
	appCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	otelConfig := otelSDK.Load()
	tel, err := otelSDK.NewTelemetry(ctx, *otelConfig)
	if err != nil {
		logger.Fatal("error while otel setup", "error", err)
	}
	defer func() {
		if err := tel.Shutdown(ctx); err != nil {
			logger.Fatal("otel shutdown error", "error", err)
		}
	}()

	// The repository ports; DATABASE_DRIVER picks the adapter behind them.
	database, err := factory.ConnectDB(ctx, factory.Config{Driver: config.DatabaseDriver, URL: config.DatabaseURL})
	if err != nil {
		logger.Fatal("error opening database", "error", err)
	}
	defer database.Close()
	logger.Logger(ctx).Info("connected to database")

	redisOptions := &redis.Options{Addr: "localhost:6379"}
	if config.RedisURL != "" {
		redisOptions, err = redis.ParseURL(config.RedisURL)
		if err != nil {
			logger.Fatal("invalid redis url", "error", err)
		}
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	// Job queue. nil when JOB_DRIVER is unset; the store is also nil on memory.
	var (
		broker        job.Broker
		jobStore      job.JobStore
		scheduleStore job.ScheduleStore
	)
	jobs, err := factory.ConnectJob(ctx, factory.JobConfig{
		Driver:    config.JobDriver,
		BrokerURL: config.JobBrokerURL,
		StoreURL:  config.JobStoreURL,
		RedisURL:  config.RedisURL,
	})
	if err != nil {
		logger.Fatal("connect job queue", "error", err)
	}
	if jobs != nil {
		defer jobs.Close()
		broker = jobs.Broker
		jobStore = jobs.Store
		scheduleStore = jobs.Schedules
	}

	// Document bodies live in Postgres. Local disk stays available for anyone
	// who wants it, but the default keeps receipts inside the same backup and
	// the same failure domain as the rows that reference them.
	var documentStore storage.Store
	if strings.EqualFold(config.DocumentStorage, "local") {
		documentStore, err = storage.NewLocalStore(config.UploadsDir)
		if err != nil {
			logger.Fatal("failed to init uploads storage", "error", err)
		}
		logger.Logger(ctx).Info("documents stored on local disk", "dir", config.UploadsDir)
	} else {
		if database.Pool == nil {
			logger.Fatal("DOCUMENT_STORAGE=postgres needs DATABASE_DRIVER=postgres; set DOCUMENT_STORAGE=local")
		}
		documentStore, err = storage.NewPostgresStore(database.Pool)
		if err != nil {
			logger.Fatal("failed to init document storage", "error", err)
		}
		logger.Logger(ctx).Info("documents stored in postgres")
	}

	pipelines, err := pipeline.Load()
	if err != nil {
		logger.Fatal("invalid pipeline definitions", "error", err)
	}

	llmCredentialsBox, err := utils.NewSecretBox(config.LLMCredentialsKey)
	if err != nil {
		// Saving AI providers is refused until the key is set; nothing else depends on it.
		logger.Logger(ctx).Warn("user AI providers disabled", "error", err)
		llmCredentialsBox = nil
	}

	cipher := serviceClient(config.CipherServiceName, config.CipherServiceURL, config.InternalAuthToken)
	var ingest *transport.Client
	if config.IngestServiceURL != "" {
		ingest = serviceClient(config.IngestServiceName, config.IngestServiceURL, config.InternalAuthToken)
	}
	cipherClient := service.NewCipherClient(cipher)

	// Services
	authService := service.NewAuthService(service.AuthConfig{
		JWTSecret:             config.JWTSecret,
		GoogleClientID:        config.GoogleClientID,
		GoogleClientSecret:    config.GoogleClientSecret,
		GoogleAndroidClientID: config.GoogleAndroidClientID,
	}, database.Auth, database.GoogleProvider, database.DBTxn)
	geocodeService := service.NewGeocodeService(config.NominatimURL, config.GeocodeCountryCodes)
	payeeService := service.NewPayeeService(database.Payee, database.PayeeRule, database.DBTxn)
	predictionService := service.NewPredictionService(database.Prediction, database.CipherPrediction)
	pushNotificationService := service.NewPushNotificationService(
		database.DevicePushToken,
		apiClient.NewExpoPushClient(config.ExpoPushURL),
	)
	transactionService := service.NewTransactionService(
		database.Transaction,
		database.Budget,
		database.TransactionEmbedding,
		cipherClient,
		database.Prediction,
		database.CipherPrediction,
		database.PayeeRule,
		database.Account,
		database.Payee,
		database.Category,
		service.NewMonthlyBudgetService(database.MonthlyBudget),
		geocodeService,
		database.DBTxn,
		broker,
	)
	recurringTransactionService := service.NewRecurringTransactionService(
		database.RecurringTransaction,
		transactionService,
	)
	statementService := service.NewStatementService(
		database.StatementImport,
		database.Transaction,
		database.Account,
		database.Payee,
		database.Category,
		transactionService,
		cipherClient,
		database.DBTxn,
	)
	// Extraction runs in-process; anything still "processing" was cut off.
	statementService.FailInterrupted(appCtx)

	websocketHub := websocket.NewConnectionHub()
	websocketService := service.NewWebsocketService(websocketHub)
	go websocket.NewRedisStreamListener(redisClient, websocketHub).Listen(appCtx)
	if jobs != nil && jobs.Updates != nil {
		forwarder := &service.JobUpdateForwarder{
			Source:      jobs.Updates,
			Websocket:   websocketService,
			Store:       jobStore,
			Connections: database.MailConnection,
		}
		go forwarder.Run(appCtx)
	}

	isDemoUser := service.NewDemoUserCheck(database.GoogleProvider, config.DemoMode)
	handlers := handler.Handlers{
		Account:       handler.NewAccountHandler(service.NewAccountService(database.Account, database.Payee, database.DBTxn)),
		Agent:         handler.NewAgentHandler(service.NewAgentService(service.NewCipherAgentClient(cipher), database.Agent, database.DBTxn)),
		AIModel:       handler.NewAIModelHandler(service.NewAIModelService(database.UserLLMModel, service.NewCipherModelTester(cipher), isDemoUser)),
		AIProvider:    handler.NewAIProviderHandler(service.NewAIProviderService(database.UserLLMProvider, llmCredentialsBox, isDemoUser)),
		APIKey:        handler.NewAPIKeyHandler(service.NewApiKeyService(database.APIKey)),
		Auth:          handler.NewAuthHandler(authService, config.Domain),
		Budget:        handler.NewBudgetHandler(service.NewBudgetService(database.Budget, database.Payee, database.Category, database.CategoryGroup, database.DBTxn)),
		Category:      handler.NewCategoryHandler(service.NewCategoryService(database.Category, database.MonthlyBudget, database.Transaction, cipherClient, database.DBTxn)),
		CategoryGoal:  handler.NewCategoryGoalHandler(service.NewCategoryGoalService(database.CategoryGoal)),
		CategoryGroup: handler.NewCategoryGroupHandler(service.NewCategoryGroupService(database.CategoryGroup)),
		Device:        handler.NewDeviceHandler(pushNotificationService),
		Document: handler.NewDocumentHandler(service.NewDocumentService(
			database.TransactionDocument,
			database.Transaction,
			database.Payee,
			documentStore,
		)),
		Geocode:        handler.NewGeocodeHandler(geocodeService),
		Job:            handler.NewJobHandler(service.NewJobService(jobStore, broker, database.MailConnection)),
		JobSchedule:    handler.NewJobScheduleHandler(service.NewJobScheduleService(scheduleStore, database.MailConnection)),
		LoanMetadata:   handler.NewLoanMetadataHandler(service.NewLoanMetadataService(database.LoanMetadata)),
		MailConnection: handler.NewMailConnectionHandler(service.NewMailConnectionService(ingest)),
		Onboarding:     handler.NewOnboardingHandler(service.NewOnboardingService(database.Budget, database.UserLLMModel)),
		Payee:          handler.NewPayeeHandler(payeeService),
		PipelineStep:   handler.NewPipelineStepHandler(service.NewPipelineStepService(pipelines, database.PipelineStep, database.UserLLMModel)),
		Prediction:     handler.NewPredictionHandler(predictionService),
		PredictionReview: handler.NewPredictionReviewHandler(service.NewPredictionReviewService(
			database.PredictionReview,
			database.Transaction,
			transactionService,
		)),
		RecurringTransaction: handler.NewRecurringTransactionHandler(recurringTransactionService),
		Report:               handler.NewReportHandler(service.NewReportService(database.Report)),
		Sms:                  handler.NewSmsHandler(service.NewSmsService(service.NewDecisionsClient(cipher), config.SmsDecisionsLog)),
		Statement:            handler.NewStatementHandler(statementService),
		Tag:                  handler.NewTagHandler(service.NewTagService(database.Tag)),
		Transaction:          handler.NewTransactionHandler(transactionService),
		User:                 handler.NewUserHandler(service.NewUserService(database.User)),
		Websocket:            handler.NewWebsocketHandler(websocketService),
	}
	if config.DemoMode {
		handlers.Demo = handler.NewDemoHandler(service.NewDemoService(
			authService,
			database.Auth,
			database.GoogleProvider,
			database.Budget,
			database.CategoryGroup,
			database.Category,
			database.Payee,
			database.Account,
			database.PayeeRule,
			database.DBTxn,
			database.BulkInserter,
		), config.Domain)
	}

	serviceAuth, err := factory.NewServiceAuthenticator(ctx, factory.ServiceAuthConfig{
		Tokens:   config.ServiceTokens,
		OwnToken: config.InternalAuthToken,
	})
	if err != nil {
		logger.Fatal("invalid SERVICE_TOKENS", "error", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(sharedMiddleware.StripInternalHeaders())
	router.Use(sharedMiddleware.RequestMetadata(config.ServiceName))
	router.Use(sharedMiddleware.InternalRequestAuth(config.InternalAuthToken, serviceAuth))
	router.Use(sharedMiddleware.RequestLogger())
	router.Use(otelgin.Middleware(otelConfig.ServiceName))
	router.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
			utils.HeaderAPIKey,
			utils.HeaderBudgetID,
			utils.HeaderCorrelationID,
			utils.HeaderCallerService,
			utils.HeaderOriginService,
			utils.HeaderInternalToken,
			utils.HeaderInternalService,
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	handler.Register(router, handlers, handler.Middleware{
		Authenticate: sharedMiddleware.Authenticate(factory.NewAuthenticators(
			ctx,
			factory.AuthConfig{JWTSecret: config.JWTSecret},
			database.APIKey,
			database.Auth,
		)...),
		RateLimit: middleware.RateLimitMiddleware(service.NewRateLimitService(redisClient)),
		Budget:    sharedMiddleware.BudgetIdMiddleware(database.Budget),
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go service.RunRecurringSchedule(appCtx, recurringTransactionService)

	// Job worker: the message pipeline's last steps. Off without a job queue.
	messageJobService := service.NewMessageJobService(service.MessageJobDeps{
		Transactions: transactionService,
		Payees:       payeeService,
		Predictions:  predictionService,
		Websocket:    websocketService,
		Push:         pushNotificationService,
		DBTxn:        database.DBTxn,

		TransactionRepo:     database.Transaction,
		Accounts:            database.Account,
		Categories:          database.Category,
		CounterpartyAliases: database.CounterpartyAlias,
		TransactionSources:  database.TransactionSource,
		CipherPredictions:   database.CipherPrediction,
	})
	workerErr := make(chan error, 1)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		err := jobWorker.Setup(appCtx, jobWorker.Deps{
			Queues:   config.JobQueues,
			Broker:   broker,
			Messages: messageJobService,
		})
		if err != nil {
			workerErr <- err
		}
	}()

	go func() {
		if err := router.Run("0.0.0.0:" + config.Port); err != nil && err != http.ErrServerClosed {
			logger.Logger(ctx).Error("http server error", "error", err)
		}
	}()

	select {
	case <-quit:
	case err := <-workerErr:
		logger.Logger(ctx).Error("unexpected worker error", "error", err)
	}
	cancel()
	logger.Logger(ctx).Info("server shutdown")

	// Let running jobs finish before the deferred Close calls.
	select {
	case <-workerDone:
	case <-time.After(workerShutdownTimeout):
		logger.Logger(ctx).Error("job worker didn't stop in time")
	}
}
