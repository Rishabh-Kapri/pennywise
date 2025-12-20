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

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5000", "http://localhost:5173", "http://192.168.1.34:5100", "https://pennywise-fe-production.up.railway.app"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Budget-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

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



	{
		api := router.Group("/api")
		api.GET("", healthPage) // simple health check
		api.GET("/category-groups", categoryGroupHandler.List)
		api.GET("/categories", categoryHandler.List)
		api.POST("/categories", categoryHandler.Create)
		api.GET("/accounts", accountHandler.List)
		api.POST("/accounts", accountHandler.Create)
	}
	router.Run("0.0.0.0:5151")
}
