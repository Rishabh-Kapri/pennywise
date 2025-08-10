package utils

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func performRequestWithHeader(headerValue string) (context.Context, error) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("GET", "/", nil)
	if headerValue != "" {
		req.Header.Set(BUDGET_ID_HEADER, headerValue)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	ctx, err := GetBudgetId(c)
	return ctx, err
}

func TestGetBudgetId_MissingBudgetId(t *testing.T) {
	ctx, err := performRequestWithHeader("")
	if err == nil || err.Error() != "Missing budgetId in context" {
		t.Errorf("Expected error to be Missing budgetId in context, got %s", err)
	}
	if ctx != nil {
		t.Errorf("Expected context to be nil when error occurs, got %v", ctx)
	}
}

func TestGetBudgetId_InvalidBudgetId(t *testing.T) {
	ctx, err := performRequestWithHeader("invalid")
	if err == nil || err.Error() != "Please enter a valid budgetId" {
		t.Errorf("Expected error to be Invalid budgetId, got %s", err)
	}
	if ctx != nil {
		t.Errorf("Expected context to be nil when error occurs, got %v", ctx)
	}
}

func TestGetBudgetId_ValidBudgetId(t *testing.T) {
	ctx, err := performRequestWithHeader("aab2f424-cb49-40f2-aa52-c07fb625961d")
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	if ctx == nil {
		t.Errorf("Expected context to be non-nil, got nil")
	}
	if ctx.Value("budgetId") == nil {
		t.Errorf("Expected context to have budgetId, got nil")
	}
}
