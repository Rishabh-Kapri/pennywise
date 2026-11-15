package middleware

import (
	"errors"
	"net/http"
	"slices"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const budgetIDHeader = utils.HeaderBudgetID

// BudgetIdMiddleware reads the budget from X-Budget-ID (or ?budgetId), checks
// that the user is one of its members, and puts the budget and the user's
// role on the context. A viewer may only read: any other method is refused.
// Verified internal requests skip the membership check.
func BudgetIdMiddleware(members repository.BudgetMemberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		log := logger.Logger(ctx)

		budgetId := c.GetHeader(utils.HeaderBudgetID)
		if budgetId == "" {
			budgetId = c.Query("budgetId")
		}
		log.Debug("checking budget membership", budgetIDHeader, budgetId)
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

		role, err := members.Role(ctx, parsedBudgetId, userID)
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied to this budget"})
			c.Abort()
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify budget access"})
			c.Abort()
			return
		}
		if !role.CanWrite() && !isReadMethod(c.Request.Method) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only view this budget", "code": "BUDGET_READ_ONLY"})
			c.Abort()
			return
		}

		ctx = utils.WithBudgetID(ctx, parsedBudgetId)
		ctx = utils.WithBudgetRole(ctx, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// RequireBudgetRole lets a request through only when the user's role in the
// budget is one of roles. It runs after BudgetIdMiddleware. Verified internal
// requests pass: they carry no user.
func RequireBudgetRole(roles ...model.BudgetRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if utils.VerifiedInternalFromContext(ctx) {
			c.Next()
			return
		}
		if !slices.Contains(roles, utils.BudgetRoleFromContext(ctx)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "only the budget's owner can do this", "code": "BUDGET_OWNER_ONLY"})
			c.Abort()
			return
		}
		c.Next()
	}
}
