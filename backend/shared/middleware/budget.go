package middleware

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func BudgetIdMiddleware(budgetRepo repository.BudgetRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		log := logger.Logger(ctx)

		budgetId := c.GetHeader(utils.HeaderBudgetID)
		if budgetId == "" {
			budgetId = c.Query("budgetId")
		}
		log.Debug("checking budget ownership", utils.HeaderBudgetID, budgetId)
		if budgetId == "" {
			abortJSON(c, http.StatusBadRequest, "missing X-Budget-ID header")
			return
		}

		parsedBudgetId, err := uuid.Parse(budgetId)
		if err != nil {
			abortJSON(c, http.StatusBadRequest, "invalid budget ID")
			return
		}

		if utils.VerifiedInternalFromContext(ctx) {
			ctx = utils.WithBudgetID(ctx, parsedBudgetId)
			log.Debug("setting budget ID in context for internal service", "budgetId", parsedBudgetId)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		userID, err := utils.UserIDFromContext(ctx)
		if err != nil {
			abortJSON(c, http.StatusUnauthorized, "user not authenticated")
			return
		}

		// Verify the budget belongs to the authenticated user
		owned, err := budgetRepo.IsOwnedByUser(ctx, parsedBudgetId, userID)
		if err != nil {
			abortJSON(c, http.StatusInternalServerError, "failed to verify budget ownership")
			return
		}
		if !owned {
			abortJSON(c, http.StatusForbidden, "access denied to this budget")
			return
		}

		ctx = utils.WithBudgetID(ctx, parsedBudgetId)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
