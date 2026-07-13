package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var monthKeyRegex = regexp.MustCompile(`^\d{4}-\d{2}$`)

type ReportHandler interface {
	GetSpending(c *gin.Context)
	GetIncomeExpense(c *gin.Context)
	GetNetWorth(c *gin.Context)
}

type reportHandler struct {
	service service.ReportService
}

func NewReportHandler(service service.ReportService) ReportHandler {
	return &reportHandler{service: service}
}

func (h *reportHandler) GetSpending(c *gin.Context) {
	params, ok := parseReportParams(c)
	if !ok {
		return
	}
	report, err := h.service.GetSpending(c.Request.Context(), params)
	if err != nil {
		respondReportError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *reportHandler) GetIncomeExpense(c *gin.Context) {
	params, ok := parseReportParams(c)
	if !ok {
		return
	}
	report, err := h.service.GetIncomeExpense(c.Request.Context(), params)
	if err != nil {
		respondReportError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *reportHandler) GetNetWorth(c *gin.Context) {
	params, ok := parseReportParams(c)
	if !ok {
		return
	}
	report, err := h.service.GetNetWorth(c.Request.Context(), params)
	if err != nil {
		respondReportError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

func parseReportParams(c *gin.Context) (model.ReportParams, bool) {
	ctx := c.Request.Context()
	utils.MustBudgetID(ctx)

	params := model.ReportParams{
		StartMonth: c.Query("startMonth"),
		EndMonth:   c.Query("endMonth"),
	}
	if params.StartMonth != "" && !monthKeyRegex.MatchString(params.StartMonth) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid startMonth, expected YYYY-MM"})
		return params, false
	}
	if params.EndMonth != "" && !monthKeyRegex.MatchString(params.EndMonth) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid endMonth, expected YYYY-MM"})
		return params, false
	}

	var err error
	if params.AccountIDs, err = parseUUIDList(c.Query("accountIds")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accountIds"})
		return params, false
	}
	if params.CategoryIDs, err = parseUUIDList(c.Query("categoryIds")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid categoryIds"})
		return params, false
	}
	return params, true
}

func parseUUIDList(value string) ([]uuid.UUID, error) {
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	ids := make([]uuid.UUID, 0, len(parts))
	for _, part := range parts {
		id, err := uuid.Parse(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func respondReportError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrInvalidReportRange) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
