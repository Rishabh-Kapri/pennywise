/*
 * Cipher: An auto prediction service for pennywise
 *
 * Handles all prediction logic, orchestrates calls to Ollama and MLP.
 * Serves as a proxy to Ollama and MLP, and stores transaction embeddings in PostgreSQL.
 * Updates transaction embeddings on user corrections.
 */
package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/memory"
	agent "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/runtime"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/tools"
	aliasAdapter "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/alias"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/resolver"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/userllm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/worker"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	sharedFactory "github.com/Rishabh-Kapri/pennywise/backend/shared/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

const shutdownTimeout = 10 * time.Second

func setupLogger(service string) {
	logger.Setup(service)
}

func healthPage(c *gin.Context) {
	c.String(http.StatusOK, "cipher Health OK!")
}

func main() {
	cfg := config.Load()
	setupLogger(cfg.ServiceName)

	ctx, stop := signal.NotifyContext(utils.WithInternalAuthToken(
		utils.WithServiceName(context.Background(), "cipher"),
		cfg.InternalAuthToken,
	), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log := logger.Logger(ctx)

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
	database, err := sharedFactory.ConnectDB(
		ctx,
		sharedFactory.Config{Driver: cfg.DatabaseDriver, URL: cfg.DatabaseURL},
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer database.Close()

	redisOptions := &redis.Options{Addr: "localhost:6379"}
	if cfg.RedisURL != "" {
		parsedOptions, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Fatal("invalid redis url", "error", err)
		}
		redisOptions = parsedOptions
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	// Clients
	// currently we only have http transport
	ollamaEngine := httpclient.NewHttpTransport(cfg.OllamaURL)
	ollamaHttpTransport := transport.NewClient("ollama", ollamaEngine)
	ollamaClient := client.NewOllamaClient(ollamaHttpTransport, tel.Tracer)

	mlpEngine := httpclient.NewHttpTransport(cfg.MLPServiceURL)
	mlpHttpTransport := transport.NewClient("mlp", mlpEngine)
	mlpClient := client.NewMLPClient(mlpHttpTransport)

	pennywiseEngine := httpclient.NewHttpTransport(cfg.PennywiseServiceURL)
	pennywiseHttpTransport := transport.NewClient("pennywise", pennywiseEngine)

	// Service

	// Read-only pool for the agent's query tools. When AGENT_DB_URL names a
	// least-privilege role, row-level security confines every agent read to the
	// caller's budget; without it we fall back to the app connection, where those
	// policies do not apply.
	agentReadDB := database.Pool
	if cfg.AgentReadOnlyDatabaseURL != "" {
		agentReadDB, err = sharedFactory.OpenSQLPool(
			ctx,
			sharedFactory.Config{Driver: cfg.DatabaseDriver, URL: cfg.AgentReadOnlyDatabaseURL},
		)
		if err != nil {
			logger.Fatal("error connecting to agent read-only database", "error", err)
		}
		defer agentReadDB.Close()
	} else {
		logger.Logger(ctx).Warn(
			"AGENT_DB_URL is not set: agent SQL tools run on the application role, " +
				"so budget isolation depends on query correctness rather than row-level security",
		)
	}

	// Registration order is the order tools are sent to the provider, and it must
	// stay stable for prompt caching. Specific tools come before execute_sql so
	// the model sees the purpose-built options first.
	agentLocation := time.Local
	if cfg.AgentTimezone != "" {
		loaded, err := time.LoadLocation(cfg.AgentTimezone)
		if err != nil {
			logger.Fatal("invalid AGENT_TIMEZONE", "timezone", cfg.AgentTimezone, "error", err)
		}
		agentLocation = loaded
	}

	toolRegistry := tools.NewToolRegistry()
	toolRegistry.RegisterTool(tools.NewGetTodayTool(agentLocation))
	toolRegistry.RegisterTool(tools.NewGetBudgetInfoTool(agentReadDB))
	toolRegistry.RegisterTool(tools.NewGetSpendingSummaryTool(agentReadDB))
	toolRegistry.RegisterTool(tools.NewGetTopTransactionsTool(agentReadDB))
	toolRegistry.RegisterTool(tools.NewGetSchemaTool())
	toolRegistry.RegisterTool(tools.NewExecuteSQLTool(agentReadDB))
	toolRegistry.RegisterTool(tools.NewUpdateWorkingMemoryTool(database.AgentMemory))

	// Every chat model call (pipeline, statements, agent) runs on the budget
	// owner's own AI providers. There is no server-key fallback.
	llmCredentialsBox, err := utils.NewSecretBox(cfg.LLMCredentialsKey)
	if err != nil {
		logger.Logger(ctx).Warn("user AI provider keys can't be opened", "error", err)
		llmCredentialsBox = nil
	}
	safeHTTPClient := transport.NewUserURLClient(cfg.AllowPrivateLLMURLs)
	userModelsResolver := resolver.NewResolver(llmCredentialsBox, safeHTTPClient, tel)
	llmResolver := userllm.New(
		database.UserLLMModel,
		database.UserLLMProvider,
		llmCredentialsBox,
		tel,
		cfg.AllowPrivateLLMURLs,
	)
	llmProviderHandler := handler.NewLLMProviderHandler(llmResolver)
	memoryService := memory.NewMemoryService(database.AgentMemory, llmResolver, agentLocation)

	agent, err := agent.NewAgent(
		llmResolver,
		toolRegistry,
		agent.WithTelemetry(tel),
		agent.WithRedis(redisClient),
		agent.WithPennywiseAPI(pennywiseHttpTransport),
		agent.WithMemory(memoryService),
		agent.WithTitleModel(cfg.AgentTitleModel),
		agent.WithMaxTurns(cfg.AgentMaxTurns),
		agent.WithMaxToolCalls(cfg.AgentMaxToolCalls),
	)
	if err != nil {
		logger.Fatal("error while creating agent", "error", err)
	}

	predictionService := service.NewPredictionService(
		agent,
		llmResolver,
		ollamaClient,
		mlpClient,
		database.TransactionEmbedding,
		database.Account,
		database.Payee,
		database.PayeeRule,
		database.Category,
		tel.Tracer,
		database.UserLLMModel,
		userModelsResolver,
	)

	agentService := service.NewAgentService(redisClient, agent, pennywiseHttpTransport, memoryService, llmResolver)

	// Handler
	predictionHandler := handler.NewPredictionHandler(predictionService)
	agentHandler := handler.NewAgentHandler(agentService)

	// Router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(sharedMiddleware.StripInternalHeaders())
	router.Use(sharedMiddleware.RequestMetadata("cipher"))
	serviceAuth, err := sharedFactory.NewServiceAuthenticator(ctx, sharedFactory.ServiceAuthConfig{
		Tokens:   cfg.ServiceTokens,
		OwnToken: cfg.InternalAuthToken,
	})
	if err != nil {
		logger.Fatal("invalid SERVICE_TOKENS", "error", err)
	}
	router.Use(sharedMiddleware.InternalRequestAuth(cfg.InternalAuthToken, serviceAuth))
	router.Use(sharedMiddleware.InternalUserIDMiddleware())
	router.Use(sharedMiddleware.RequestLogger())
	router.Use(otelgin.Middleware(otelConfig.ServiceName))
	// router.Use(tel.LogRequest())
	// router.Use(tel.MeterRequestDuration())
	// router.Use(tel.MeterRequestsInFlight())

	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:5173"},
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			utils.HeaderInternalService,
			utils.HeaderInternalToken,
			utils.HeaderBudgetID,
			utils.HeaderCorrelationID,
			utils.HeaderCallerService,
			utils.HeaderOriginService,
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	{
		api := router.Group("/api")
		api.GET("", healthPage)

		budgetApi := api.Group("")
		budgetApi.Use(sharedMiddleware.BudgetIdMiddleware(database.BudgetMember))
		budgetApi.POST("/email/normalize", predictionHandler.NormalizeEmailText)
		budgetApi.POST("/email/extract", predictionHandler.ExtractEmailData)
		budgetApi.POST("/predict", predictionHandler.Predict)
		budgetApi.POST("/embeddings/transaction", predictionHandler.GenerateTransactionEmbedding)
		budgetApi.POST("/corrections", predictionHandler.HandleCorrection)
		budgetApi.POST("/categories/describe", predictionHandler.SuggestCategoryDescription)
		budgetApi.POST("/statements/extract", predictionHandler.ExtractStatement)
		budgetApi.POST("/llm/models/test", llmProviderHandler.TestModel)

		{
			agentApi := api.Group("/agent")
			agentApi.Use(sharedMiddleware.BudgetIdMiddleware(database.BudgetMember))
			agentApi.GET("/runs", agentHandler.GetRun)
			agentApi.POST("/runs", agentHandler.CreateRun)
			agentApi.GET("/runs/cancel/:id", agentHandler.CancelRun)
		}
	}

	addr := "0.0.0.0:" + cfg.Port
	log.Info("cipher listening on", "addr", addr)

	// setup server with graceful shutdown
	serverErr := make(chan error, 1)
	srv := &http.Server{
		Addr:    addr,
		Handler: router.Handler(),
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
	}()

	// setup worker
	var broker job.Broker // nil: no queue, so ingestion answers 503
	jobs, err := sharedFactory.ConnectJob(ctx, sharedFactory.JobConfig{
		Driver:    cfg.JobDriver,
		BrokerURL: cfg.JobBrokerURL,
		StoreURL:  cfg.JobStoreURL,
		RedisURL:  cfg.RedisURL,
	})
	if err != nil {
		logger.Fatal("connect job queue", "err", err)
	}
	if jobs != nil {
		defer jobs.Close()
		broker = jobs.Broker
	}

	messagePipeline, err := factory.NewMessagePipeline(
		database.PipelineStep,
		database.UserLLMModel,
		database.Category,
		userModelsResolver,
	)
	statementImport := factory.NewStatementFactory(
		factory.OCRConfig{
			Driver:    cfg.StatementImportDriver,
			Provider:  cfg.StatementImportProvider,
			Model:     cfg.StatementImportModel,
			MaxTokens: cfg.StatementImportMaxTokens,
		},
		userModelsResolver,
		database.UserLLMModel,
	)
	if err != nil {
		logger.Fatal("error building message pipeline", "err", err)
	}

	workerErr := make(chan error, 1)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		err := worker.Setup(ctx, worker.WorkerDeps{
			Queues:   cfg.JobQueues,
			Broker:   broker,
			Pipeline: messagePipeline,
			Memory:   aliasAdapter.NewLookup(database.CounterpartyAlias),

			ParkLowConfidence: cfg.MessageReviewPark,
			StatementImport:   statementImport,
		})
		if err != nil {
			workerErr <- err
			return
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("server shutdown")
	case e := <-serverErr:
		if e != nil {
			log.Error("error starting server", "err", e)
		}
	case e := <-workerErr:
		if e != nil && !errors.Is(e, context.Canceled) {
			log.Error("unexpected worker error", "err", e)
		}
	}
	stop()

	// wait for a timeout for active connections to close
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", "err", err)
	}

	select {
	case <-workerDone:
		log.Info("worker done")
	case <-shutdownCtx.Done():
		log.Info("worker didn't shutdown on time")
	}
}
