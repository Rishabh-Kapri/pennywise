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

	userRepo := repository.NewUserRepository(dbConn)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	payeeRepo := repository.NewPayeesRepository(dbConn)
	payeeService := service.NewPayeeService(payeeRepo)
	payeeHandler := handler.NewPayeeHandler(payeeService)

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
		{
			accountGroup := router.Group("/api/accounts")
			accountGroup.GET("/search", accountHandler.Search)
			accountGroup.GET("", accountHandler.List)
			accountGroup.POST("", accountHandler.Create)
		}
		{
			userGroup := router.Group("/api/users")
			userGroup.GET("/search", userHandler.Search)
			userGroup.PATCH("", userHandler.Update)
		}
		{
			groupGroup := router.Group("/api/category-groups")
			groupGroup.GET("", categoryGroupHandler.List)
			groupGroup.POST("", categoryGroupHandler.Create)
			groupGroup.PUT(":id", categoryGroupHandler.Update)
			groupGroup.DELETE(":id", categoryGroupHandler.DeleteById)
		}
		{
			categoryGroup := router.Group("/api/categories")
			categoryGroup.POST("", categoryHandler.Create)
			categoryGroup.GET("", categoryHandler.List)
			categoryGroup.GET("/search", categoryHandler.Search)
			categoryGroup.GET(":id", categoryHandler.GetById)
			categoryGroup.PUT(":id", categoryHandler.Update)
			categoryGroup.DELETE(":id", categoryHandler.DeleteById)
		}
		{
			transactionGroup := router.Group("/api/transactions")
			transactionGroup.GET("", transactionHandler.List)
			transactionGroup.GET("/normalized", transactionHandler.ListNormalized)
			transactionGroup.POST("", transactionHandler.Create)
			transactionGroup.PATCH(":id", transactionHandler.Update)
			transactionGroup.DELETE(":id", transactionHandler.DeleteById)
		}
		{
			payeeGroup := router.Group("/api/payees")
			payeeGroup.GET("", payeeHandler.List)
			payeeGroup.GET("/search", payeeHandler.Search)
			payeeGroup.POST("", payeeHandler.Create)
			payeeGroup.PATCH(":id", payeeHandler.Update)
			payeeGroup.DELETE(":id", payeeHandler.DeleteById)
		}
		{
			predictionGroup := router.Group("/api/predictions")
			predictionGroup.GET("", predictionHandler.List)
			predictionGroup.POST("", predictionHandler.Create)
			predictionGroup.PATCH(":id", predictionHandler.Update)
			predictionGroup.DELETE(":id", predictionHandler.DeleteById)
		}
	}
	router.Run("0.0.0.0:5151")
}
