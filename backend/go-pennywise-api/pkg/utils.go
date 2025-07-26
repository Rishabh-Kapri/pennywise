package utils

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
)

const (
	BUDGET_ID_HEADER = "X-Budget-ID"
)

/**
* Take gin context and return the budgetId, checks for error and returns it in context
 */
func GetBudgetId(c *gin.Context) (context.Context, error) {
	budgetId := c.GetHeader(BUDGET_ID_HEADER)
	if budgetId == "" {
		return nil, errors.New("Missing budgetId in context")
	}
	ctx := context.WithValue(c.Request.Context(), "budgetId", budgetId)
	return ctx, nil
}
