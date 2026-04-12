package main

import (
	"log"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/service"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func setupLogger() {
	logger.Setup("orchestrator")
}

func healthPage(c *gin.Context) {
	c.String(http.StatusOK, "Orchestrator Health OK!")
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
	ollamaClient := client.NewOllamaClient(cfg.OllamaURL)
	mlpClient := client.NewMLPClient(cfg.MLPApiURL)

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
	log.Printf("Orchestrator listening on %s\n", addr)
	router.Run(addr)
}
