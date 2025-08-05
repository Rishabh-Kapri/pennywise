package main

import (
	"net/http"

	"pennywise-api/internal/db"
	"pennywise-api/internal/handler"
	"pennywise-api/internal/repository"
	"pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

func healthPage(c *gin.Context) {
	c.String(http.StatusOK, "Health OK!")
}

func main() {
	dbConn := db.Connect()
	router := gin.Default()

	defer dbConn.Close()

	accountRepo := repository.NewAccountRepository(dbConn)
	accountService := service.NewAccountService(accountRepo)
	accountHandler := handler.NewAccountHandler(accountService)

	categoryGroupRepo := repository.NewCategoryGroupRepository(dbConn)
	categoryGroupService := service.NewCategoryGroupService(categoryGroupRepo)
	categoryGroupHandler := handler.NewCategoryGroupHandler(categoryGroupService)

	categoryRepo := repository.NewCategoryRepository(dbConn)
	categoryService := service.NewCategoryService(categoryRepo)
	categoryHandler := handler.NewCategoryHandler(categoryService)

	transactionRepo := repository.NewTransactionRepository(dbConn)
	transactionService := service.NewTransactionService(transactionRepo)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	predictionRepo := repository.NewPredictionRepository(dbConn)
	predictionService := service.NewPredictionService(predictionRepo)
	predictionHandler := handler.NewPredictionHandler(predictionService)

	{
		api := router.Group("/api")
		api.GET("", healthPage) // simple health check
		api.GET("/category-groups", categoryGroupHandler.List)
		{
			categoryGroup := router.Group("/api/categories")
			categoryGroup.POST("", categoryHandler.Create)
			categoryGroup.GET("", categoryHandler.List)
			categoryGroup.GET(":id", categoryHandler.GetById)
			categoryGroup.PUT(":id", categoryHandler.Update)
			categoryGroup.DELETE(":id", categoryHandler.DeleteById)
		}
		{
			transactionGroup := router.Group("/api/transactions")
			transactionGroup.GET("", transactionHandler.List)
		}
		api.GET("/predictions", predictionHandler.List)
		api.POST("/predictions", predictionHandler.Create)

		api.GET("/accounts", accountHandler.List)
		api.POST("/accounts", accountHandler.Create)
	}
	router.Run("0.0.0.0:5151")
}
