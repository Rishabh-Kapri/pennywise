package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/db"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	temporalActivities "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/temporal/activities"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/websocket"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
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

	dbConn := db.Connect(ctx)
	defer dbConn.Close()
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

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(sharedMiddleware.StripInternalHeaders())
	router.Use(sharedMiddleware.RequestMetadata(config.ServiceName))
	router.Use(sharedMiddleware.InternalRequestAuth(config.InternalAuthToken))
	router.Use(sharedMiddleware.RequestLogger())

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

	budgetRepo := repository.NewBudgetRepository(dbConn)
	payeeRepo := repository.NewPayeesRepository(dbConn)
	payeeRuleRepo := repository.NewPayeeRuleRepository(dbConn)
	categoryRepo := repository.NewCategoryRepository(dbConn)
	categoryGroupRepo := repository.NewCategoryGroupRepository(dbConn)
	predictionRepo := repository.NewPredictionRepository(dbConn)
	cipherPredictionRepo := repository.NewCipherPredictionRepository(dbConn)
	accountRepo := repository.NewAccountRepository(dbConn)
	userRepo := repository.NewUserRepository(dbConn)
	transactionRepo := repository.NewTransactionRepository(dbConn)
	embeddingRepo := repository.NewEmbeddingRepository(dbConn)
	transactionEmbeddingRepo := repository.NewTransactionEmbeddingRepository(dbConn)
	tagRepo := repository.NewTagRepository(dbConn)
	authRepo := repository.NewAuthRepository(dbConn)
	googleProviderRepo := repository.NewGoogleProviderRepository(dbConn)
	apiKeyRepo := repository.NewAPIKeyRepository(dbConn)

	budgetService := service.NewBudgetService(budgetRepo, payeeRepo, categoryRepo, categoryGroupRepo)
	budgetHandler := handler.NewBudgetHandler(budgetService)

	accountService := service.NewAccountService(accountRepo, payeeRepo)
	accountHandler := handler.NewAccountHandler(accountService)

	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	payeeService := service.NewPayeeService(payeeRepo)
	payeeHandler := handler.NewPayeeHandler(payeeService)

	categoryGroupService := service.NewCategoryGroupService(categoryGroupRepo)
	categoryGroupHandler := handler.NewCategoryGroupHandler(categoryGroupService)

	monthlyBudgetRepo := repository.NewMonthlyBudgetRepository(dbConn)
	monthlyBudgetService := service.NewMonthlyBudgetService(monthlyBudgetRepo)

	predictionService := service.NewPredictionService(predictionRepo, cipherPredictionRepo)
	predictionHandler := handler.NewPredictionHandler(predictionService)

	cipherHttpTransport := httpclient.NewHttpTransport(config.CipherServiceURL)
	cipherTransportClient := transport.NewClient(config.CipherServiceName, cipherHttpTransport)
	cipherClient := service.NewCipherClient(cipherTransportClient)

	transactionService := service.NewTransactionService(
		transactionRepo,
		budgetRepo,
		transactionEmbeddingRepo,
		cipherClient,
		predictionRepo,
		cipherPredictionRepo,
		payeeRuleRepo,
		accountRepo,
		payeeRepo,
		categoryRepo,
		monthlyBudgetService,
	)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	categoryService := service.NewCategoryService(categoryRepo, monthlyBudgetRepo, transactionRepo)
	categoryHandler := handler.NewCategoryHandler(categoryService)

	embeddingService := service.NewEmbeddingService(embeddingRepo)
	embeddingHandler := handler.NewEmbeddingHandler(embeddingService)

	tagService := service.NewTagService(tagRepo)
	tagHandler := handler.NewTagHandler(tagService)

	gmailHttpTransport := httpclient.NewHttpTransport(config.GmailServiceURL)
	gmailClient := transport.NewClient(config.GmailServiceName, gmailHttpTransport)
	authService := service.NewAuthService(authRepo, googleProviderRepo, gmailClient)
	authHandler := handler.NewAuthHandler(authService)

	apiKeyService := service.NewApiKeyService(apiKeyRepo)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService)

	loanMetadataRepo := repository.NewLoanMetadataRepository(dbConn)
	loanMetadataService := service.NewLoanMetadataService(loanMetadataRepo)
	loanMetadataHandler := handler.NewLoanMetadataHandler(loanMetadataService)

	websocketHub := websocket.NewConnectionHub()
	websocketService := service.NewWebsocketService(websocketHub)
	websocketHandler := handler.NewWebsocketHandler(websocketService)
	go websocketHub.HandleBroadcastMessages() // run once

	// Auth middleware
	authMiddleware := middleware.AuthMiddleware(authService, apiKeyService)
	rateLimitMiddleware := middleware.RateLimitMiddleware(service.NewRateLimitService(redisClient))
	budgetMiddleware := sharedMiddleware.BudgetIdMiddleware(budgetRepo)

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
			apiKeyGroup := router.Group("/api/keys")
			apiKeyGroup.Use(authMiddleware, rateLimitMiddleware)
			apiKeyGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), apiKeyHandler.GetByKeyID)
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
			accountGroup := router.Group("/api/accounts")
			accountGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			accountGroup.GET("/search", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), accountHandler.Search)
			accountGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), accountHandler.List)
			accountGroup.POST("", middleware.RouteAuthMiddleware(sharedModel.ScopeWrite), accountHandler.Create)
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
			transactionGroup.DELETE(
				":id",
				middleware.RouteAuthMiddleware(sharedModel.ScopeDelete),
				transactionHandler.DeleteById,
			)
		}
		{
			payeeGroup := router.Group("/api/payees")
			payeeGroup.Use(authMiddleware, rateLimitMiddleware, budgetMiddleware)
			payeeGroup.GET("", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), payeeHandler.List)
			payeeGroup.GET("/search", middleware.RouteAuthMiddleware(sharedModel.ScopeRead), payeeHandler.Search)
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
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Temporal worker — skipped if TEMPORAL_SERVER_HOST is not set
	if config.Environment != "local" && config.TemporalServerHost != "" {
		temporalClient, err := client.Dial(client.Options{
			HostPort:           fmt.Sprintf("%s:%s", config.TemporalServerHost, config.TemporalServerPort),
			ContextPropagators: sharedTemporal.ContextPropagators(),
		})
		if err != nil {
			logger.Logger(ctx).Error("failed to create temporal client", "error", err)
			panic(err)
		}
		_, err = temporalClient.ScheduleClient().Create(ctx, client.ScheduleOptions{
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
			DB:                 dbConn,
		})
		w.RegisterActivity(&temporalActivities.CreateCipherPredictionActivity{
			PredictionService: predictionService,
		})
		w.RegisterActivity(&temporalActivities.FetchGoogleUsersActivity{
			AuthService: authService,
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

	go func() {
		if err := router.Run("0.0.0.0:5151"); err != nil && err != http.ErrServerClosed {
			logger.Logger(ctx).Error("http server error", "error", err)
		}
	}()

	<-quit
	logger.Logger(ctx).Info("server shutdown")
}
