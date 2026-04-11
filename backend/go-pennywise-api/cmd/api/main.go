package main

import (
	"net/http"

	"pennywise-api/internal/db"
	"pennywise-api/internal/handler"
	"pennywise-api/internal/middleware"
	"pennywise-api/internal/repository"
	"pennywise-api/internal/service"
	utils "pennywise-api/pkg"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func healthPage(c *gin.Context) {
	c.String(http.StatusOK, "Health OK!")
}

func main() {
	utils.SetupLogger()

	dbConn := db.Connect()
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5000", "http://localhost:5173", "http://192.168.1.34:5100", "https://pennywise-fe-production.up.railway.app", "https://react-fe-production-8fe5.up.railway.app", "https://pennywise.nastydomain.space", "https://react-fe-dev.up.railway.app"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Budget-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	defer dbConn.Close()
	budgetRepo := repository.NewBudgetRepository(dbConn)
	payeeRepo := repository.NewPayeesRepository(dbConn)
	categoryRepo := repository.NewCategoryRepository(dbConn)
	categoryGroupRepo := repository.NewCategoryGroupRepository(dbConn)
	predictionRepo := repository.NewPredictionRepository(dbConn)
	accountRepo := repository.NewAccountRepository(dbConn)
	userRepo := repository.NewUserRepository(dbConn)
	transactionRepo := repository.NewTransactionRepository(dbConn)
	embeddingRepo := repository.NewEmbeddingRepository(dbConn)
	tagRepo := repository.NewTagRepository(dbConn)
	authRepo := repository.NewAuthRepository(dbConn)

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

	predictionService := service.NewPredictionService(predictionRepo)
	predictionHandler := handler.NewPredictionHandler(predictionService)

	transactionService := service.NewTransactionService(
		transactionRepo,
		budgetRepo,
		predictionRepo,
		accountRepo,
		payeeRepo,
		categoryRepo,
		monthlyBudgetRepo,
	)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	categoryService := service.NewCategoryService(categoryRepo, monthlyBudgetRepo, transactionRepo)
	categoryHandler := handler.NewCategoryHandler(categoryService)

	embeddingService := service.NewEmbeddingService(embeddingRepo)
	embeddingHandler := handler.NewEmbeddingHandler(embeddingService)

	tagService := service.NewTagService(tagRepo)
	tagHandler := handler.NewTagHandler(tagService)

	authService := service.NewAuthService(authRepo)
	authHandler := handler.NewAuthHandler(authService)

	loanMetadataRepo := repository.NewLoanMetadataRepository(dbConn)
	loanMetadataService := service.NewLoanMetadataService(loanMetadataRepo)
	loanMetadataHandler := handler.NewLoanMetadataHandler(loanMetadataService)

	// Auth middleware
	authMiddleware := middleware.AuthMiddleware(authService)
	budgetMiddleware := middleware.BudgetIdMiddleware(budgetRepo)

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
			budgetGroup := router.Group("/api/budgets")
			budgetGroup.Use(authMiddleware)
			budgetGroup.GET("", budgetHandler.List)
			budgetGroup.POST("", budgetHandler.Create)
			budgetGroup.PATCH(":id", budgetHandler.UpdateById)
		}
		{
			accountGroup := router.Group("/api/accounts")
			accountGroup.Use(authMiddleware, budgetMiddleware)
			accountGroup.GET("/search", accountHandler.Search)
			accountGroup.GET("", accountHandler.List)
			accountGroup.POST("", accountHandler.Create)
		}
		{
			userGroup := router.Group("/api/users")
			userGroup.Use(authMiddleware, budgetMiddleware)
			userGroup.GET("/search", userHandler.Search)
			userGroup.PATCH("", userHandler.Update)
		}
		{
			groupGroup := router.Group("/api/category-groups")
			groupGroup.Use(authMiddleware, budgetMiddleware)
			groupGroup.GET("", categoryGroupHandler.List)
			groupGroup.POST("", categoryGroupHandler.Create)
			groupGroup.PUT(":id", categoryGroupHandler.Update)
			groupGroup.DELETE(":id", categoryGroupHandler.DeleteById)
		}
		{
			categoryGroup := router.Group("/api/categories")
			categoryGroup.Use(authMiddleware, budgetMiddleware)
			categoryGroup.POST("", categoryHandler.Create)
			categoryGroup.GET("", categoryHandler.List)
			categoryGroup.GET("/inflow", categoryHandler.GetInflowBalance)
			categoryGroup.PATCH("/:id/:month", categoryHandler.UpdateBudget)
			categoryGroup.GET("/search", categoryHandler.Search)
			categoryGroup.GET(":id", categoryHandler.GetById)
			categoryGroup.PUT(":id", categoryHandler.Update)
			categoryGroup.DELETE(":id", categoryHandler.DeleteById)
		}
		{
			transactionGroup := router.Group("/api/transactions")
			transactionGroup.Use(authMiddleware, budgetMiddleware)
			transactionGroup.GET("", transactionHandler.List)
			transactionGroup.GET("/normalized", transactionHandler.ListNormalized)
			transactionGroup.POST("", transactionHandler.Create)
			transactionGroup.PATCH(":id", transactionHandler.Update)
			transactionGroup.DELETE(":id", transactionHandler.DeleteById)
		}
		{
			payeeGroup := router.Group("/api/payees")
			payeeGroup.Use(authMiddleware, budgetMiddleware)
			payeeGroup.GET("", payeeHandler.List)
			payeeGroup.GET("/search", payeeHandler.Search)
			payeeGroup.POST("", payeeHandler.Create)
			payeeGroup.PATCH(":id", payeeHandler.Update)
			payeeGroup.DELETE(":id", payeeHandler.DeleteById)
		}
		{
			tagGroup := router.Group("/api/tags")
			tagGroup.Use(authMiddleware, budgetMiddleware)
			tagGroup.GET("", tagHandler.List)
			tagGroup.GET("/search", tagHandler.Search)
			tagGroup.POST("", tagHandler.Create)
			tagGroup.PATCH(":id", tagHandler.Update)
			tagGroup.DELETE(":id", tagHandler.DeleteById)
		}
		{
			predictionGroup := router.Group("/api/predictions")
			predictionGroup.Use(authMiddleware, budgetMiddleware)
			predictionGroup.GET("", predictionHandler.List)
			predictionGroup.POST("", predictionHandler.Create)
			predictionGroup.PATCH(":id", predictionHandler.Update)
			predictionGroup.DELETE(":id", predictionHandler.DeleteById)
		}
		{
			embeddingGroup := router.Group("/api/embeddings")
			embeddingGroup.Use(authMiddleware)
			embeddingGroup.POST("", embeddingHandler.Create)
			embeddingGroup.GET("/search", embeddingHandler.Search)
		}
		{
			loanMetadataGroup := router.Group("/api/loan-metadata")
			loanMetadataGroup.Use(authMiddleware, budgetMiddleware)
			loanMetadataGroup.GET("", loanMetadataHandler.List)
			loanMetadataGroup.GET(":accountId", loanMetadataHandler.GetByAccountId)
			loanMetadataGroup.POST("", loanMetadataHandler.Create)
			loanMetadataGroup.PATCH(":accountId", loanMetadataHandler.Update)
			loanMetadataGroup.DELETE(":accountId", loanMetadataHandler.Delete)
		}
	}
	router.Run("0.0.0.0:5151")
}
