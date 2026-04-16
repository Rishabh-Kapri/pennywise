/*
 * Cipher: An auto prediction service for pennywise
 *
 * Handles all prediction logic, orchestrates calls to Ollama and MLP.
 * Serves as a proxy to Ollama and MLP, and stores transaction embeddings in PostgreSQL.
 * Updates transaction embeddings on user corrections.
 */
package main

import (
	"log"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"

	"github.com/gin-contrib/cors"
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
	ollamaClient := client.NewOllamaClient(ollamaHttpTransport)

	mlpEngine := httpclient.NewHttpTransport(cfg.MLPApiURL)
	mlpHttpTransport := transport.NewClient("mlp", mlpEngine)
	mlpClient := client.NewMLPClient(mlpHttpTransport)

	// Repository
	txnEmbeddingRepo := repository.NewTransactionEmbeddingRepository(dbConn)

	// Service
	predictionService := service.NewPredictionService(ollamaClient, mlpClient, txnEmbeddingRepo)

	// Handler
	predictionHandler := handler.NewPredictionHandler(predictionService)

	// Router
	router := gin.New()
	router.Use(gin.Recovery())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Type", "X-Budget-ID", "X-Correlation-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	{
		api := router.Group("/api")
		api.GET("", healthPage)
		api.POST("/predict", predictionHandler.Predict)
		api.POST("/corrections", predictionHandler.HandleCorrection)
	}

	addr := "0.0.0.0:" + cfg.Port
	log.Printf("cipher listening on %s\n", addr)
	router.Run(addr)
}
