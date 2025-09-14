package main

import (
	"net/http"

	"pennywise-api/internal/db"
	"pennywise-api/internal/handler"
	"pennywise-api/internal/repository"
	"pennywise-api/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func healthPage(c *gin.Context) {
	c.String(http.StatusOK, "Health OK!")
}

func main() {
	dbConn := db.Connect()
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5000"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Budget-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	defer dbConn.Close()

	budgetRepo := repository.NewBudgetRepository(dbConn)
	budgetService := service.NewBudgetService(budgetRepo)
	budgetHandler := handler.NewBudgetHandler(budgetService)

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
	monthlyBudgetRepo := repository.NewMonthlyBudgetRepository(dbConn)
	categoryService := service.NewCategoryService(categoryRepo, monthlyBudgetRepo)
	categoryHandler := handler.NewCategoryHandler(categoryService)

	predictionRepo := repository.NewPredictionRepository(dbConn)
	predictionService := service.NewPredictionService(predictionRepo)
	predictionHandler := handler.NewPredictionHandler(predictionService)

	transactionRepo := repository.NewTransactionRepository(dbConn)
	transactionService := service.NewTransactionService(transactionRepo, predictionRepo, accountRepo, payeeRepo, categoryRepo, monthlyBudgetRepo)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	embeddingRepo := repository.NewEmbeddingRepository(dbConn)
	embeddingService := service.NewEmbeddingService(embeddingRepo)
	embeddingHandler := handler.NewEmbeddingHandler(embeddingService)

	{
		api := router.Group("/api")
		api.GET("", healthPage) // simple health check
		{
			budgetGroup := router.Group("/api/budgets")
			budgetGroup.GET("", budgetHandler.List)
		}
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
			categoryGroup.PATCH("/:id/:month", categoryHandler.UpdateBudget)
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
		{
			embeddingGroup := router.Group("/api/embeddings")
			embeddingGroup.POST("", embeddingHandler.Create)
			embeddingGroup.GET("/search", embeddingHandler.Search)
		}
	}
	router.Run("0.0.0.0:5151")
}
