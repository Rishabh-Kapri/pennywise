package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PipelineHandler interface {
	ListRuns(c *gin.Context)
	GetRun(c *gin.Context)
	RetryRun(c *gin.Context)
}

type pipelineHandler struct {
	service service.PipelineService
}

func NewPipelineHandler(service service.PipelineService) PipelineHandler {
	return &pipelineHandler{service: service}
}

func (h *pipelineHandler) ListRuns(c *gin.Context) {
	ctx := c.Request.Context()

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	status := c.Query("status")

	runs, err := h.service.GetRuns(ctx, status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if runs == nil {
		runs = []model.PipelineRun{}
	}
	c.JSON(http.StatusOK, runs)
}

func (h *pipelineHandler) GetRun(c *gin.Context) {
	ctx := c.Request.Context()

	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run id"})
		return
	}

	detail, err := h.service.GetRunDetail(ctx, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *pipelineHandler) RetryRun(c *gin.Context) {
	ctx := c.Request.Context()

	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run id"})
		return
	}

	run, err := h.service.RetryRun(ctx, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "retry signal sent", "run": run})
}
