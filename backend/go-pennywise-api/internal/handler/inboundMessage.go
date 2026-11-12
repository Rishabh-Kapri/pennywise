package handler

import (
	"net/http"
	"strconv"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
)

type InboundMessageHandler interface {
	List(c *gin.Context)
}

type inboundMessageHandler struct {
	service service.InboundMessageService
}

func NewInboundMessageHandler(service service.InboundMessageService) InboundMessageHandler {
	return &inboundMessageHandler{service: service}
}

// List is GET /api/messages?status=needs_review&limit=50. Status defaults to
// needs_review: the messages waiting for the user.
func (h *inboundMessageHandler) List(c *gin.Context) {
	status := model.InboundMessageStatus(c.DefaultQuery("status", string(model.InboundMessageNeedsReview)))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	list, err := h.service.List(c.Request.Context(), status, limit)
	if err != nil {
		if errs.HasCode(err, errs.CodeInvalidArgument) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}
