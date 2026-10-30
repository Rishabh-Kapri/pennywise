package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"

	apiClient "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/storage"
	temporalActivities "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/temporal/activities"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/websocket"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	sharedTemporal "github.com/Rishabh-Kapri/pennywise/backend/shared/temporal"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func healthPage(c *gin.Context) {
	c.String(http.StatusOK, "Health OK!")
}

func main() {
	config := config.Load()
	logger.Setup(config.ServiceName)
	ctx := utils.WithInternalAuthToken(
		utils.WithServiceName(context.Background(), config.ServiceName),
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
	database, err := factory.ConnectDB(factory.Config{Driver: config.DatabaseDriver, URL: config.DatabaseURL})
	if err != nil {
		logger.Fatal("error opening database", "error", err)
	}
	defer database.Close()
	logger.Logger(ctx).Info("connected to database")

	redisOptions := &redis.Options{Addr: "localhost:6379"}
	if config.RedisURL != "" {
		parsedOptions, err := redis.ParseURL(config.RedisURL)
		if err != nil {
			logger.Logger(ctx).Error("invalid redis url", "error", err)
			panic(err)
		}
		redisOptions = parsedOptions
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	// Temporal client — nil when Temporal is not configured (e.g. local dev).
	// Shared by the pipeline retry endpoint and the activity worker below.
	var temporalClient client.Client
	if config.Environment != "local" && config.TemporalServerHost != "" {
		tcl, err := client.Dial(client.Options{
			HostPort:           fmt.Sprintf("%s:%s", config.TemporalServerHost, config.TemporalServerPort),
			ContextPropagators: sharedTemporal.ContextPropagators(),
		})
		if err != nil {
			logger.Logger(ctx).Error("failed to create temporal client", "error", err)
			panic(err)
		}
		temporalClient = tcl
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(sharedMiddleware.StripInternalHeaders())
	router.Use(sharedMiddleware.RequestMetadata(config.ServiceName))
	router.Use(sharedMiddleware.InternalRequestAuth(config.InternalAuthToken))
	router.Use(sharedMiddleware.RequestLogger())
	router.Use(otelgin.Middleware(otelConfig.ServiceName))

	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5000",
			"http://localhost:5173",
			"http://192.168.1.34:5100",
			"https://pennywise-fe-production.up.railway.app",
			"https://react-fe-production-8fe5.up.railway.app",
			"https://dev.pennywise.cloud",
			"https://pennywise.cloud",
			"https://www.pennywise.cloud",
			"https://pennywise.nastydomain.space",
			"https://react-fe-dev.up.railway.app",
		},
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

	budgetService := service.NewBudgetService(
		database.Budget,
		database.Payee,
		database.Category,
		database.CategoryGroup,
		database.DBTxn,
	)
	budgetHandler := handler.NewBudgetHandler(budgetService)

	accountService := service.NewAccountService(database.Account, database.Payee, database.DBTxn)
	accountHandler := handler.NewAccountHandler(accountService)

	userService := service.NewUserService(database.User)
	userHandler := handler.NewUserHandler(userService)

	payeeService := service.NewPayeeService(database.Payee, database.PayeeRule, database.DBTxn)
	payeeHandler := handler.NewPayeeHandler(payeeService)

	categoryGroupService := service.NewCategoryGroupService(database.CategoryGroup)
	categoryGroupHandler := handler.NewCategoryGroupHandler(categoryGroupService)

	monthlyBudgetService := service.NewMonthlyBudgetService(database.MonthlyBudget)

	predictionService := service.NewPredictionService(database.Prediction, database.CipherPrediction)
	predictionHandler := handler.NewPredictionHandler(predictionService)

	cipherHttpTransport := httpclient.NewHttpTransport(config.CipherServiceURL)
	cipherDefaultHeaders := map[string][]string{}
	if config.InternalAuthToken != "" {
		cipherDefaultHeaders[utils.HeaderInternalToken] = []string{config.InternalAuthToken}
	}
	cipherTransportClient := transport.NewClient(
		config.CipherServiceName,
		cipherHttpTransport,
		transport.WithDefaultHeaders(cipherDefaultHeaders),
	)
	cipherClient := service.NewCipherClient(cipherTransportClient)
	agentClient := service.NewCipherAgentClient(cipherTransportClient)
	agentService := service.NewAgentService(agentClient, database.Agent, database.DBTxn)
	agentHandler := handler.NewAgentHandler(agentService)

	geocodeService := service.NewGeocodeService(config.NominatimURL, config.GeocodeCountryCodes)
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
		monthlyBudgetService,
		geocodeService,
		database.DBTxn,
	)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	// Document bodies live in Postgres. Local disk stays available for anyone
	// who wants it, but the default keeps receipts inside the same backup and
	// the same failure domain as the rows that reference them.
	var documentStore storage.Store
	if strings.EqualFold(config.DocumentStorage, "local") {
		documentStore, err = storage.NewLocalStore(config.UploadsDir)
		if err != nil {
			logger.Logger(ctx).Error("failed to init uploads storage", "error", err)
			panic(err)
		}
		logger.Logger(ctx).Info("documents stored on local disk", "dir", config.UploadsDir)
	} else {
		if database.Pool == nil {
			logger.Fatal("DOCUMENT_STORAGE=postgres needs DATABASE_DRIVER=postgres; set DOCUMENT_STORAGE=local")
		}
		documentStore, err = storage.NewPostgresStore(database.Pool)
		if err != nil {
			logger.Logger(ctx).Error("failed to init document storage", "error", err)
			panic(err)
		}
		logger.Logger(ctx).Info("documents stored in postgres")
	}

	documentService := service.NewDocumentService(
		database.TransactionDocument,
		database.Transaction,
		database.Payee,
		documentStore,
	)
	documentHandler := handler.NewDocumentHandler(documentService)

	pushNotificationService := service.NewPushNotificationService(
		database.DevicePushToken,
		apiClient.NewExpoPushClient(config.ExpoPushURL),
	)
	deviceHandler := handler.NewDeviceHandler(pushNotificationService)
	geocodeHandler := handler.NewGeocodeHandler(geocodeService)

	categoryService := service.NewCategoryService(
		database.Category,
		database.MonthlyBudget,
		database.Transaction,
		cipherClient,
		database.DBTxn,
	)
	categoryHandler := handler.NewCategoryHandler(categoryService)
	categoryGoalHandler := handler.NewCategoryGoalHandler(
		service.NewCategoryGoalService(database.CategoryGoal),
	)

	embeddingService := service.NewEmbeddingService(database.Embedding)
	embeddingHandler := handler.NewEmbeddingHandler(embeddingService)

	tagService := service.NewTagService(database.Tag)
	tagHandler := handler.NewTagHandler(tagService)

	gmailHttpTransport := httpclient.NewHttpTransport(config.GmailServiceURL)
	gmailClient := transport.NewClient(config.GmailServiceName, gmailHttpTransport)
	authService := service.NewAuthService(database.Auth, database.GoogleProvider, gmailClient, database.DBTxn)
	authHandler := handler.NewAuthHandler(authService)

	onboardingHandler := handler.NewOnboardingHandler(
		service.NewOnboardingService(
			database.Budget,
			database.Auth,
			database.GoogleProvider,
			database.UserLLMProvider,
			gmailClient,
		),
	)
	llmCredentialsBox, err := utils.NewSecretBox(config.LLMCredentialsKey)
	if err != nil {
		// Saving AI providers is refused until the key is set; nothing else depends on it.
		logger.Logger(ctx).Warn("user AI providers disabled", "error", err)
		llmCredentialsBox = nil
	}
	aiProviderHandler := handler.NewAIProviderHandler(service.NewAIProviderService(
		database.UserLLMProvider,
		llmCredentialsBox,
		cipherTransportClient,
		service.NewDemoUserCheck(database.GoogleProvider, config.DemoMode),
	))

	var demoHandler handler.DemoHandler
	if config.DemoMode {
		demoService := service.NewDemoService(
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
		)
		demoHandler = handler.NewDemoHandler(demoService)
	}

	apiKeyService := service.NewApiKeyService(database.APIKey)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService)

	loanMetadataService := service.NewLoanMetadataService(database.LoanMetadata)
	loanMetadataHandler := handler.NewLoanMetadataHandler(loanMetadataService)

	reportService := service.NewReportService(database.Report)
	reportHandler := handler.NewReportHandler(reportService)

	pipelineService := service.NewPipelineService(database.PipelineRun, temporalClient)
	pipelineHandler := handler.NewPipelineHandler(pipelineService)

	recurringTransactionService := service.NewRecurringTransactionService(
		database.RecurringTransaction,
		transactionService,
	)
	recurringTransactionHandler := handler.NewRecurringTransactionHandler(recurringTransactionService)

	predictionReviewService := service.NewPredictionReviewService(
		database.PredictionReview,
		database.Transaction,
		transactionService,
	)
	predictionReviewHandler := handler.NewPredictionReviewHandler(predictionReviewService)

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
	statementHandler := handler.NewStatementHandler(statementService)

	websocketHub := websocket.NewConnectionHub()
	websocketService := service.NewWebsocketService(websocketHub)
	websocketHandler := handler.NewWebsocketHandler(websocketService)
	// go websocketHub.HandleBroadcastMessages() // run once
	go websocket.NewRedisStreamListener(redisClient, websocketHub).Listen(appCtx)

	// Auth middleware
	authMiddleware := middleware.AuthMiddleware(authService, apiKeyService)
	rateLimitMiddleware := middleware.RateLimitMiddleware(service.NewRateLimitService(redisClient))
	budgetMiddleware := sharedMiddleware.BudgetIdMiddleware(database.Budget)

	// Websocket routes
	{
		ws := router.Group("/ws")
		ws.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
		ws.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), websocketHandler.Connect)
		ws.GET("/sessions", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), websocketHandler.GetSessions)
		ws.POST("/test-event", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), websocketHandler.SendTestEvent)
	}
	{
		api := router.Group("/api")
		api.GET("", healthPage) // simple health check

		// Public auth routes (no auth required)
		{
			authGroup := router.Group("/api/auth")
			authGroup.POST("/google", authHandler.LoginWithGoogle)
			authGroup.POST("/refresh", authHandler.RefreshToken)
			if config.DemoMode {
				authGroup.POST("/demo", demoHandler.LoginAsDemo)
			}
			// authGroup.POST("/logout", authHandler.Logout)
		}

		// Protected auth routes
		// {
		// 	authProtected := router.Group("/api/auth")
		// 	authProtected.Use(authMiddleware)
		// 	authProtected.GET("/me", authHandler.GetCurrentUser)
		// 	authProtected.POST("/logout-all", authHandler.LogoutAll)
		// }

		// Protected routes - all require authentication
		{
			authUserGroup := router.Group("/api/auth/users")
			authUserGroup.Use(authMiddleware, rateLimitMiddleware)
			authUserGroup.GET("/me", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), authHandler.GetCurrentUser)
		}
		{
			// push tokens are user-scoped, not budget-scoped
			deviceGroup := router.Group("/api/devices")
			deviceGroup.Use(authMiddleware, rateLimitMiddleware)
			deviceGroup.POST(
				"/push-token",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				deviceHandler.RegisterPushToken,
			)
			deviceGroup.DELETE(
				"/push-token",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				deviceHandler.UnregisterPushToken,
			)
		}
		{
			// No budget middleware: place lookup is not budget-scoped data.
			geocodeGroup := router.Group("/api/geocode")
			geocodeGroup.Use(authMiddleware, rateLimitMiddleware)
			geocodeGroup.GET(
				"/search",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				geocodeHandler.Search,
			)
		}
		{
			agentGroup := router.Group("/api/agent")
			agentGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			agentGroup.GET(
				"/conversations",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				agentHandler.GetConversations,
			)
			agentGroup.GET(
				"/conversations/:id/messages",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				agentHandler.GetConversationMessages,
			)
			agentGroup.PATCH(
				"/conversations/:conversationId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				agentHandler.UpdateConversation,
			)
			agentGroup.DELETE(
				"/conversations/:conversationId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				agentHandler.DeleteConversation,
			)
			agentGroup.PATCH(
				"/conversations/:conversationId/message/:messageId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				agentHandler.UpdateConversationMessageContent,
			)
			agentGroup.POST("/runs", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), agentHandler.CreateRun)
			agentGroup.GET("/runs/:id", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), agentHandler.GetRun)
			agentGroup.POST(
				"/runs/:id/cancel",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				agentHandler.CancelRun,
			)
			agentGroup.PATCH(
				"/:entity/:id/metadata",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				agentHandler.UpdateEntityMetadata,
			)
		}
		{
			apiKeyGroup := router.Group("/api/keys")
			apiKeyGroup.Use(authMiddleware, rateLimitMiddleware)
			apiKeyGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), apiKeyHandler.GetAll)
			apiKeyGroup.GET("/:keyID", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), apiKeyHandler.GetByKeyID)
			apiKeyGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeAdmin), apiKeyHandler.Create)
		}
		{
			budgetGroup := router.Group("/api/budgets")
			budgetGroup.Use(authMiddleware, rateLimitMiddleware)
			budgetGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), budgetHandler.List)
			budgetGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), budgetHandler.Create)
			budgetGroup.PATCH("/:id", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), budgetHandler.UpdateById)
		}
		// Auth-only provider user routes (no budget middleware) — used by internal services
		{
			providerUserGroup := router.Group("/api/auth/:provider/users")
			providerUserGroup.Use(authMiddleware, rateLimitMiddleware)
			providerUserGroup.GET(
				"",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				authHandler.GetProviderUser,
			)
			providerUserGroup.PATCH(
				"",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				authHandler.UpdateProviderUser,
			)
		}
		{
			onboardingGroup := router.Group("/api/onboarding")
			onboardingGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			onboardingGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), onboardingHandler.Get)
			onboardingGroup.PATCH("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), onboardingHandler.Update)
			onboardingGroup.POST(
				"/email-backfill",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				onboardingHandler.BackfillEmails,
			)
		}
		{
			// Providers belong to the user; the budget header is only needed
			// because cipher resolves a test call through the budget owner.
			aiProviderGroup := router.Group("/api/ai-providers")
			aiProviderGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			aiProviderGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), aiProviderHandler.List)
			aiProviderGroup.PUT(
				"/:provider",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				aiProviderHandler.Upsert,
			)
			aiProviderGroup.DELETE(
				"/:provider",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				aiProviderHandler.Delete,
			)
			aiProviderGroup.POST(
				"/:provider/test",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				aiProviderHandler.Test,
			)
		}
		{
			accountGroup := router.Group("/api/accounts")
			accountGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			accountGroup.GET("/search", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), accountHandler.Search)
			accountGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), accountHandler.List)
			accountGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), accountHandler.Create)
			accountGroup.PATCH("/:id", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), accountHandler.Update)
		}
		{
			userGroup := router.Group("/api/users")
			userGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			userGroup.GET("/search", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), userHandler.Search)
			userGroup.PATCH("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), userHandler.Update)
		}
		{
			groupGroup := router.Group("/api/category-groups")
			groupGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			groupGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), categoryGroupHandler.List)
			groupGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), categoryGroupHandler.Create)
			groupGroup.PUT(":id", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), categoryGroupHandler.Update)
			groupGroup.DELETE(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				categoryGroupHandler.DeleteById,
			)
		}
		{
			categoryGroup := router.Group("/api/categories")
			categoryGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			categoryGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), categoryHandler.Create)
			categoryGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), categoryHandler.List)
			categoryGroup.GET(
				"/inflow",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				categoryHandler.GetInflowBalance,
			)
			categoryGroup.PATCH(
				"/:id/:month",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				categoryHandler.UpdateBudget,
			)
			categoryGroup.GET("/search", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), categoryHandler.Search)
			categoryGroup.GET(":id", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), categoryHandler.GetById)
			categoryGroup.PUT(":id", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), categoryHandler.Update)
			categoryGroup.PUT(
				":id/details",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				categoryHandler.UpdateDetails,
			)
			categoryGroup.POST(
				":id/description/suggest",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				categoryHandler.SuggestDescription,
			)
			categoryGroup.PUT(
				":id/goal",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				categoryGoalHandler.Upsert,
			)
			categoryGroup.DELETE(
				":id/goal",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				categoryGoalHandler.Delete,
			)
			categoryGroup.DELETE(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				categoryHandler.DeleteById,
			)
		}
		{
			transactionGroup := router.Group("/api/transactions")
			transactionGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			transactionGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), transactionHandler.List)
			transactionGroup.GET(
				"/normalized",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				transactionHandler.ListNormalized,
			)
			transactionGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), transactionHandler.Create)
			transactionGroup.PATCH(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				transactionHandler.Update,
			)
			transactionGroup.PATCH(
				":id/status",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				transactionHandler.UpdateStatus,
			)
			transactionGroup.PATCH(
				":id/location",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				transactionHandler.UpdateLocation,
			)
			transactionGroup.DELETE(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				transactionHandler.DeleteById,
			)
			transactionGroup.POST(
				":id/documents",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				documentHandler.Upload,
			)
			transactionGroup.POST(
				":id/documents/scan",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				documentHandler.UploadScan,
			)
			transactionGroup.GET(
				":id/documents",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				documentHandler.ListByTransaction,
			)
		}
		{
			documentGroup := router.Group("/api/documents")
			documentGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			documentGroup.GET(
				"",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				documentHandler.List,
			)
			documentGroup.GET(
				":docId/content",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				documentHandler.Content,
			)
			documentGroup.DELETE(
				":docId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				documentHandler.Delete,
			)
		}
		{
			statementGroup := router.Group("/api/statements")
			statementGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			statementGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), statementHandler.List)
			statementGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), statementHandler.Upload)
			statementGroup.GET(":id", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), statementHandler.Get)
			statementGroup.PATCH(
				":id/sections/:section",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				statementHandler.SetSectionAccount,
			)
			statementGroup.POST(
				":id/transactions",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				statementHandler.CreateMissing,
			)
			statementGroup.POST(
				":id/retry",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				statementHandler.Retry,
			)
			statementGroup.DELETE(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				statementHandler.Delete,
			)
		}
		{
			payeeGroup := router.Group("/api/payees")
			payeeGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			payeeGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), payeeHandler.List)
			payeeGroup.GET("/search", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), payeeHandler.Search)
			payeeGroup.GET("/stats", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), payeeHandler.Stats)
			payeeGroup.GET("/rules", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), payeeHandler.ListRules)
			payeeGroup.POST("/merge", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), payeeHandler.Merge)
			payeeGroup.GET("/:id", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), payeeHandler.GetById)
			payeeGroup.GET("/:id/rules", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), payeeHandler.GetRules)
			payeeGroup.POST(
				"/:id/rules",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				payeeHandler.CreateRule,
			)
			payeeGroup.PATCH(
				"/:id/rules/:ruleId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				payeeHandler.UpdateRule,
			)
			payeeGroup.DELETE(
				"/:id/rules/:ruleId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				payeeHandler.DeleteRule,
			)
			payeeGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), payeeHandler.Create)
			payeeGroup.PATCH(":id", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), payeeHandler.Update)
			payeeGroup.DELETE(":id", middleware.RouteAuthMiddleware(sharedModel.ScopeDelete), payeeHandler.DeleteById)
		}
		{
			tagGroup := router.Group("/api/tags")
			tagGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			tagGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), tagHandler.List)
			tagGroup.GET("/search", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), tagHandler.Search)
			tagGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), tagHandler.Create)
			tagGroup.PATCH(":id", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), tagHandler.Update)
			tagGroup.DELETE(":id", middleware.RouteAuthMiddleware(sharedModel.ScopeDelete), tagHandler.DeleteById)
		}
		{
			predictionGroup := router.Group("/api/predictions")
			predictionGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			predictionGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), predictionHandler.List)
			predictionGroup.GET(
				"/cipher",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				predictionHandler.ListCipherPredictions,
			)
			predictionGroup.GET(
				"/transactions/:transactionId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				predictionHandler.GetByTransactionID,
			)
			predictionGroup.GET(
				"/review",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				predictionReviewHandler.Queue,
			)
			predictionGroup.POST(
				"/review/:id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				predictionReviewHandler.Review,
			)
			predictionGroup.GET(
				"/learning",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				transactionHandler.LearningStatus,
			)
			predictionGroup.POST(
				"/learning/backfill",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				transactionHandler.BackfillLearning,
			)
			predictionGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), predictionHandler.Create)
			predictionGroup.PATCH(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				predictionHandler.Update,
			)
			predictionGroup.DELETE(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				predictionHandler.DeleteById,
			)
		}
		{
			embeddingGroup := router.Group("/api/embeddings")
			embeddingGroup.Use(authMiddleware, rateLimitMiddleware)
			embeddingGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), embeddingHandler.Create)
			embeddingGroup.GET(
				"/search",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				embeddingHandler.Search,
			)
		}
		{
			loanMetadataGroup := router.Group("/api/loan-metadata")
			loanMetadataGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			loanMetadataGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), loanMetadataHandler.List)
			loanMetadataGroup.GET(
				":accountId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				loanMetadataHandler.GetByAccountId,
			)
			loanMetadataGroup.POST(
				"",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				loanMetadataHandler.Create,
			)
			loanMetadataGroup.PATCH(
				":accountId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				loanMetadataHandler.Update,
			)
			loanMetadataGroup.DELETE(
				":accountId",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				loanMetadataHandler.Delete,
			)
		}
		{
			recurringGroup := router.Group("/api/recurring-transactions")
			recurringGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			recurringGroup.GET(
				"",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				recurringTransactionHandler.List,
			)
			recurringGroup.POST(
				"",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				recurringTransactionHandler.Create,
			)
			recurringGroup.POST(
				"/run",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				recurringTransactionHandler.Run,
			)
			recurringGroup.PATCH(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				recurringTransactionHandler.Update,
			)
			recurringGroup.DELETE(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				recurringTransactionHandler.DeleteById,
			)
		}
		{
			reportGroup := router.Group("/api/reports")
			reportGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			reportGroup.GET(
				"/spending",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				reportHandler.GetSpending,
			)
			reportGroup.GET(
				"/income-expense",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				reportHandler.GetIncomeExpense,
			)
			reportGroup.GET(
				"/networth",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				reportHandler.GetNetWorth,
			)
			reportGroup.GET(
				"/cashflow",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				reportHandler.GetCashFlow,
			)
			reportGroup.GET(
				"/loans",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				reportHandler.GetLoanHistories,
			)
		}
		{
			pipelineGroup := router.Group("/api/pipeline")
			pipelineGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			pipelineGroup.GET(
				"/runs",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				pipelineHandler.ListRuns,
			)
			pipelineGroup.GET(
				"/runs/:id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeRead),
				pipelineHandler.GetRun,
			)
			pipelineGroup.POST(
				"/runs/:id/retry",
				middleware.RouteAuthMiddleware(sharedModel.ScopeWrite),
				pipelineHandler.RetryRun,
			)
		}
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Recurring transactions scheduler. Deliberately a plain ticker rather
	// than a Temporal schedule: the Temporal worker below is skipped in local
	// and Docker-only setups, and recurring transactions must still fire
	// there. Materialization is driven by each rule's next_date, so a missed
	// tick (or a restart) is caught up on the following pass.
	go func() {
		schedulerCtx := utils.WithInternalAuthToken(
			utils.WithServiceName(appCtx, config.ServiceName),
			config.InternalAuthToken,
		)
		runRecurring := func() {
			result, err := recurringTransactionService.RunDueAllBudgets(schedulerCtx)
			if err != nil {
				logger.Logger(schedulerCtx).Error("recurring transactions run failed", "error", err)
				return
			}
			if result.Created > 0 || result.Skipped > 0 {
				logger.Logger(schedulerCtx).Info(
					"recurring transactions run complete",
					"created", result.Created,
					"skipped", result.Skipped,
				)
			}
		}

		// let the server finish coming up before the first pass
		select {
		case <-time.After(30 * time.Second):
			runRecurring()
		case <-schedulerCtx.Done():
			return
		}

		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runRecurring()
			case <-schedulerCtx.Done():
				return
			}
		}
	}()

	// Temporal worker — skipped if TEMPORAL_SERVER_HOST is not set
	if temporalClient != nil {
		_, err := temporalClient.ScheduleClient().Create(ctx, client.ScheduleOptions{
			ID: "sync-gmail-watch-workflow-schedule",
			Spec: client.ScheduleSpec{
				CronExpressions: []string{"0 12 */2 * *"}, // every 2 days at 12:00 PM
				StartAt:         time.Date(2026, 4, 29, 7, 30, 0, 0, time.UTC),
			},
			Action: &client.ScheduleWorkflowAction{
				ID:        "",
				Workflow:  sharedModel.RefreshGmailWatchWorkflowName,
				TaskQueue: sharedModel.PennywiseTaskQueue,
			},
		})
		if err != nil {
			logger.Logger(ctx).Warn("failed to create gmail watch schedule", "error", err)
		}

		w := worker.New(temporalClient, sharedModel.PennywiseActivitiesTaskQueue, worker.Options{
			BackgroundActivityContext: utils.WithInternalAuthToken(
				utils.WithServiceName(context.Background(), config.ServiceName),
				config.InternalAuthToken,
			),
		})
		w.RegisterActivity(&temporalActivities.CreateTransactionActivity{
			TransactionService: transactionService,
			PayeeService:       payeeService,
			PredictionService:  predictionService,
			WebsocketService:   websocketService,
			PushService:        pushNotificationService,
			DBTxn:              database.DBTxn,
		})
		w.RegisterActivity(&temporalActivities.CreateCipherPredictionActivity{
			PredictionService: predictionService,
		})
		w.RegisterActivity(&temporalActivities.FetchGoogleUsersActivity{
			AuthService: authService,
		})
		w.RegisterActivity(&temporalActivities.PipelineStatusActivity{
			PipelineRunRepo:  database.PipelineRun,
			WebsocketService: websocketService,
			DBTxn:            database.DBTxn,
		})

		if err := w.Start(); err != nil {
			logger.Logger(ctx).Error("failed to start temporal worker", "error", err)
			panic(err)
		}
		logger.Logger(ctx).Info("temporal worker started", "taskQueue", sharedModel.PennywiseActivitiesTaskQueue)

		go func() {
			<-quit
			logger.Logger(ctx).Info("shutting down temporal worker")
			w.Stop()
			temporalClient.Close()
		}()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5151"
	}
	go func() {
		if err := router.Run("0.0.0.0:" + port); err != nil && err != http.ErrServerClosed {
			logger.Logger(ctx).Error("http server error", "error", err)
		}
	}()

	<-quit
	cancel()
	logger.Logger(ctx).Info("server shutdown")
}
