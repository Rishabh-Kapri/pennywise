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

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func setupLogger() {
	logger.Setup("cipher")
}

func healthPage(c *gin.Context) {
	c.String(http.StatusOK, "cipher Health OK!")
}

func main() {
	setupLogger()

	ctx := context.Background()

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

	cfg := config.Load()

	// Database connection via shared module
	dbConn, err := db.ConnectWithURL(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer dbConn.Close()

	// Clients
	// currently we only have http transport
	ollamaEngine := httpclient.NewHttpTransport(cfg.OllamaURL)
	ollamaHttpTransport := transport.NewClient("ollama", ollamaEngine)
	ollamaClient := client.NewOllamaClient(ollamaHttpTransport, tel.Tracer)

	mlpEngine := httpclient.NewHttpTransport(cfg.MLPServiceURL)
	mlpHttpTransport := transport.NewClient("mlp", mlpEngine)
	mlpClient := client.NewMLPClient(mlpHttpTransport)

	// Repository
	txnEmbeddingRepo := repository.NewTransactionEmbeddingRepository(dbConn)
	budgetRepo := repository.NewBudgetRepository(dbConn)
	payeeRepo := repository.NewPayeesRepository(dbConn)
	payeeRuleRepo := repository.NewPayeeRuleRepository(dbConn)
	categoryRepo := repository.NewCategoryRepository(dbConn)

	// Service
	predictionService := service.NewPredictionService(
		ollamaClient,
		mlpClient,
		txnEmbeddingRepo,
		payeeRepo,
		payeeRuleRepo,
		categoryRepo,
		tel.Tracer,
	)

	// Handler
	predictionHandler := handler.NewPredictionHandler(predictionService)

	// Router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	// router.Use(sharedMiddleware.RequestLogger())
	router.Use(otelgin.Middleware(otelConfig.ServiceName))
	router.Use(tel.LogRequest())
	router.Use(tel.MeterRequestDuration())
	router.Use(tel.MeterRequestsInFlight())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Type", "X-Internal-Service", "X-Budget-ID", "X-Correlation-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	{
		api := router.Group("/api")
		api.GET("", healthPage)

		budgetApi := api.Group("")
		budgetApi.Use(sharedMiddleware.BudgetIdMiddleware(budgetRepo))
		budgetApi.POST("/predict", predictionHandler.Predict)
		budgetApi.POST("/corrections", predictionHandler.HandleCorrection)
	}

	addr := "0.0.0.0:" + cfg.Port
	log.Printf("cipher listening on %s\n", addr)
	router.Run(addr)
}
