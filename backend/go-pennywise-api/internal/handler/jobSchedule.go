package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type JobScheduleHandler struct {
	service service.JobScheduleService
}

func NewJobScheduleHandler(service service.JobScheduleService) *JobScheduleHandler {
	return &JobScheduleHandler{service: service}
}

func (h *JobScheduleHandler) List(c *gin.Context) {
	list, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *JobScheduleHandler) Routes(r gin.IRouter) {
	r.GET("/jobs/schedules", h.List)
}
