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
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	agentContext "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/context"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm/providers"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/memory"
	agent "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/runtime"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/tools"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/temporal"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
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

	tc "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// package-level Temporal client, set in main() and used by handlers
var temporalClient tc.Client

func setupLogger() {
	logger.Setup("cipher")
}

func healthPage(c *gin.Context) {
	c.String(http.StatusOK, "cipher Health OK!")
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

func getLLMClients(tel otelSDK.TelemetryProvider) (map[string]llm.RegistryEntry, string, error) {
	appConfig := config.Load()
	entries := map[string]llm.RegistryEntry{}

	if appConfig.AnthropicAPIKey != "" {
		c, err := providers.NewAnthropicClient("chat")
		if err != nil {
			return nil, "", err
		}
		oc := llm.NewObservedLLM(c, tel)
		entries["anthropic"] = llm.RegistryEntry{Client: oc, DefaultModel: "claude-sonnet-4-6"}
	}

	if appConfig.OpenAIAPIKey != "" {
		c, err := providers.NewOpenAIClient()
		if err != nil {
			return nil, "", err
		}
		oc := llm.NewObservedLLM(c, tel)
		entries["openai"] = llm.RegistryEntry{Client: oc, DefaultModel: "gpt-4o"}
	}

	if appConfig.OpenRouterAPIKey != "" {
		c, err := providers.NewOpenRouterClient()
		if err != nil {
			return nil, "", err
		}
		oc := llm.NewObservedLLM(c, tel)
		entries["openrouter"] = llm.RegistryEntry{Client: oc, DefaultModel: "anthropic/claude-haiku-4.5"}
	}

	ollamaClient, err := providers.NewOllamaClient()
	if err != nil {
		return nil, "", err
	}
	ollamaObserved := llm.NewObservedLLM(ollamaClient, tel)
	entries["ollama"] = llm.RegistryEntry{
		Client:       ollamaObserved,
		DefaultModel: "gemma4",
	}

	defaultProvider := appConfig.DefaultAgentProvider
	if _, ok := entries[defaultProvider]; !ok {
		defaultProvider = func() string {
			for _, provider := range []string{"openai", "anthropic", "openrouter", "ollama"} {
				if _, ok := entries[provider]; ok {
					return provider
				}
			}
			return ""
		}()
	}

	if _, ok := entries[defaultProvider]; !ok {
		return nil, "", errs.New(errs.CodeInternalError,
			"default provider %q is not present in registry entries", defaultProvider)
	}

	return entries, defaultProvider, nil
}

func main() {
	setupLogger()
	cfg := config.Load()

	ctx := utils.WithInternalAuthToken(utils.WithServiceName(context.Background(), "cipher"), cfg.InternalAuthToken)

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

	// Database connection via shared module
	dbConn, err := db.ConnectWithURL(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer dbConn.Close()

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

	// Repository
	txnEmbeddingRepo := repository.NewTransactionEmbeddingRepository(dbConn)
	accountRepo := repository.NewAccountRepository(dbConn)
	budgetRepo := repository.NewBudgetRepository(dbConn)
	payeeRepo := repository.NewPayeesRepository(dbConn)
	payeeRuleRepo := repository.NewPayeeRuleRepository(dbConn)
	categoryRepo := repository.NewCategoryRepository(dbConn)
	categoryGroupRepo := repository.NewCategoryGroupRepository(dbConn)
	agentMemoryRepo := repository.NewAgentMemoryRepository(dbConn)

	// Service

	toolRegistry := tools.NewToolRegistry()
	toolRegistry.RegisterTool(tools.NewGetBudgetInfoTool(dbConn))
	toolRegistry.RegisterTool(tools.NewGetSchemaTool())
	toolRegistry.RegisterTool(tools.NewExecuteSQLTool(dbConn))
	toolRegistry.RegisterTool(tools.NewUpdateWorkingMemoryTool(dbConn))

	contextBuilder := agentContext.NewContextBuilder(
		dbConn,
		accountRepo,
		budgetRepo,
		categoryRepo,
		payeeRepo,
		categoryGroupRepo,
	)
	llmClients, defaultProvider, err := getLLMClients(tel)
	if err != nil {
		logger.Fatal("error while getting llm clients", "error", err)
	}

	llmResolver, err := llm.NewLLMRegistry(llmClients, defaultProvider, tel)
	if err != nil {
		logger.Fatal("error whiel creating llm registry", "error", err)
	}
	memoryService := memory.NewMemoryService(agentMemoryRepo, llmResolver)

	agent, err := agent.NewAgent(
		llmResolver,
		toolRegistry,
		agent.WithTelemetry(tel),
		agent.WithRedis(redisClient),
		agent.WithContextBuilder(contextBuilder),
		agent.WithPennywiseAPI(pennywiseHttpTransport),
		agent.WithMemory(memoryService),
	)
	if err != nil {
		logger.Fatal("error while creating agent", "error", err)
	}

	predictionService := service.NewPredictionService(
		agent,
		llmResolver,
		ollamaClient,
		mlpClient,
		txnEmbeddingRepo,
		accountRepo,
		payeeRepo,
		payeeRuleRepo,
		categoryRepo,
		tel.Tracer,
	)

	agentService := service.NewAgentService(redisClient, agent, pennywiseHttpTransport, memoryService, llmResolver)

	if cfg.Environment != "local" {
		temporalClient, err = connectToTemporal(ctx, cfg)
		if err != nil {
			logger.Fatal("Unable to connect to Temporal", "error", err)
		}
		defer temporalClient.Close()

		w := worker.New(temporalClient, sharedModel.CipherActivitiesTaskQueue, worker.Options{
			UseBuildIDForVersioning: false,
			BackgroundActivityContext: utils.WithInternalAuthToken(
				utils.WithServiceName(context.Background(), "cipher"),
				cfg.InternalAuthToken,
			),
		})
		w.RegisterActivity(&temporal.PredictionActivity{PredictionService: predictionService})

		go func() {
			if err := w.Run(worker.InterruptCh()); err != nil {
				logger.Fatal("Temporal activity worker failed", "error", err)
			}
		}()
	}
	// Handler
	predictionHandler := handler.NewPredictionHandler(predictionService)
	workflowHandler := handler.NewWorkflowHandler(temporalClient)
	agentHandler := handler.NewAgentHandler(agentService)

	// Router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(sharedMiddleware.StripInternalHeaders())
	router.Use(sharedMiddleware.RequestMetadata("cipher"))
	router.Use(sharedMiddleware.InternalRequestAuth(cfg.InternalAuthToken))
	router.Use(sharedMiddleware.InternalUserIDMiddleware())
	router.Use(sharedMiddleware.RequestLogger())
	router.Use(otelgin.Middleware(otelConfig.ServiceName))
	router.Use(tel.LogRequest())
	router.Use(tel.MeterRequestDuration())
	router.Use(tel.MeterRequestsInFlight())

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
		budgetApi.Use(sharedMiddleware.BudgetIdMiddleware(budgetRepo))
		budgetApi.POST("/email/normalize", predictionHandler.NormalizeEmailText)
		budgetApi.POST("/email/extract", predictionHandler.ExtractEmailData)
		budgetApi.POST("/predict", predictionHandler.Predict)
		budgetApi.POST("/embeddings/transaction", predictionHandler.GenerateTransactionEmbedding)
		budgetApi.POST("/corrections", predictionHandler.HandleCorrection)

		api.POST("/workflows/:workflowId/retry-predict", workflowHandler.RetryPredict)
		api.POST("/workflows/parsed-to-transaction", workflowHandler.StartParsedEmailToTransaction)

		{
			agentApi := api.Group("/agent")
			agentApi.Use(sharedMiddleware.BudgetIdMiddleware(budgetRepo))
			agentApi.GET("/runs", agentHandler.GetRun)
			agentApi.POST("/runs", agentHandler.CreateRun)
			agentApi.GET("/runs/cancel/:id", agentHandler.CancelRun)
		}
	}

	addr := "0.0.0.0:" + cfg.Port
	log.Printf("cipher listening on %s\n", addr)
	go router.Run(addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
