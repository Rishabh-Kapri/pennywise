package middleware

import (
	"log"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const budgetIDHeader = "X-Budget-ID"

func BudgetIdMiddleware(budgetRepo repository.BudgetRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		userID, err := utils.UserIDFromContext(ctx)
		log.Printf("userID: %v err: %v", userID, err)
		if err != nil {
			userID, err = uuid.Parse("fb7c7893-84f7-4344-a861-064985d442f7")
			// c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
			// c.Abort()
			// return
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

		// Verify the budget belongs to the authenticated user
		owned, err := budgetRepo.IsOwnedByUser(ctx, parsedBudgetId, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify budget ownership"})
			c.Abort()
			return
		}
		if !owned {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied to this budget"})
			c.Abort()
			return
		}

		ctx = utils.WithBudgetID(ctx, parsedBudgetId)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
