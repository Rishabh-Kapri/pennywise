package middleware

import (
	"net/http"

	utils "pennywise-api/pkg"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const budgetIDHeader = "X-Budget-ID"

func BudgetIdMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "userID not found in context"})
			c.Abort()
			return
		}

		budgetId := c.GetHeader(budgetIDHeader)
		if budgetId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Budget-ID header"})
			c.Abort()
			return
		}

		parsedBudgetId, err := uuid.Parse(budgetId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid budget ID"})
			c.Abort()
			return
		}

		ctx := utils.WithBudgetID(c.Request.Context(), parsedBudgetId)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
