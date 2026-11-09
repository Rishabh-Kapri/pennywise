package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// jobStatuses are the status filters the Jobs screen sends.
var jobStatuses = map[string]bool{
	"": true, "queued": true, "running": true, "retrying": true,
	"succeeded": true, "dead": true, "cancelled": true,
}

type JobHandler interface {
	List(c *gin.Context)
	Get(c *gin.Context)
	Retry(c *gin.Context)
	Cancel(c *gin.Context)
}

type jobHandler struct {
	service service.JobService
}

func NewJobHandler(service service.JobService) JobHandler {
	return &jobHandler{service: service}
}

func (h *jobHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	input := service.JobListInput{
		Status:    c.Query("status"),
		Type:      c.Query("type"),
		Queue:     c.Query("queue"),
		ErrorCode: c.Query("errorCode"),
		Limit:     limit,
		Offset:    max(offset, 0),
	}
	if !jobStatuses[input.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	if raw := c.Query("jobId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
			return
		}
		input.JobID = &id
	}
	if raw := c.Query("since"); raw != "" {
		since, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since"})
			return
		}
		input.Since = &since
	}

	list, err := h.service.List(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *jobHandler) Get(c *gin.Context) {
	id, ok := parseJobID(c)
	if !ok {
		return
	}
	detail, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *jobHandler) Retry(c *gin.Context) {
	id, ok := parseJobID(c)
	if !ok {
		return
	}
	if err := h.service.Retry(c.Request.Context(), id); err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "queued"})
}

func (h *jobHandler) Cancel(c *gin.Context) {
	id, ok := parseJobID(c)
	if !ok {
		return
	}
	if err := h.service.Cancel(c.Request.Context(), id); err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

func parseJobID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return uuid.Nil, false
	}
	return id, true
}

func writeJobError(c *gin.Context, err error) {
	switch {
	case errs.HasCode(err, errs.CodeJobNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
	case errs.HasCode(err, errs.CodeJobInvalidState):
		c.JSON(http.StatusConflict, gin.H{"error": "the job is no longer in a state that allows this"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
