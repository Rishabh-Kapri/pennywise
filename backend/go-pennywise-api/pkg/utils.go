package utils

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	if err := uuid.Validate(budgetId); err != nil {
		return nil, errors.New("Please enter a valid budgetId")
	}
	parsedBudgetId, err := uuid.Parse(budgetId)
	if err != nil {
		return nil, errors.New("Error while parsing budgetId to UUID")
	}
	ctx := context.WithValue(c.Request.Context(), "budgetId", parsedBudgetId)
	return ctx, nil
}
