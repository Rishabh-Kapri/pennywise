package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
)

// PipelineStepHandler serves the budget's pipeline settings
// (GET /pipelines, PUT and DELETE /pipelines/:pipeline/steps/:step).
type PipelineStepHandler interface {
	List(c *gin.Context)
	Update(c *gin.Context)
	Reset(c *gin.Context)
}

type pipelineStepHandler struct {
	service service.PipelineStepService
}

func NewPipelineStepHandler(service service.PipelineStepService) PipelineStepHandler {
	return &pipelineStepHandler{service: service}
}

func writePipelineStepError(c *gin.Context, err error) {
	var e *errs.Error
	switch {
	case errors.Is(err, service.ErrUnknownPipelineStep):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errs.HasCode(err, errs.CodePipelineInvalidOverride) && errors.As(err, &e):
		// The message is written for the user; the code is for the client.
		c.JSON(http.StatusBadRequest, gin.H{"error": e.Message, "code": e.Code})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *pipelineStepHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "budget id required"})
		return
	}
	view, err := h.service.List(ctx, budgetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *pipelineStepHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "budget id required"})
		return
	}
	var body service.UpdatePipelineStepRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	trimInPlace(body.Adapter)
	step, err := h.service.Update(ctx, budgetID, c.Param("pipeline"), pipeline.StepID(c.Param("step")), body)
	if err != nil {
		writePipelineStepError(c, err)
		return
	}
	c.JSON(http.StatusOK, step)
}

func (h *pipelineStepHandler) Reset(c *gin.Context) {
	ctx := c.Request.Context()
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "budget id required"})
		return
	}
	step, err := h.service.Reset(ctx, budgetID, c.Param("pipeline"), pipeline.StepID(c.Param("step")))
	if err != nil {
		writePipelineStepError(c, err)
		return
	}
	c.JSON(http.StatusOK, step)
}

func trimInPlace(s *string) {
	if s != nil {
		*s = strings.TrimSpace(*s)
	}
}
