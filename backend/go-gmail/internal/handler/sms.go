package handler

import (
	"net/http"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/internal/service"

	"github.com/gin-gonic/gin"
)

// SmsIngestReq is one bank SMS forwarded by the Android app
// (android-frontend/src/features/sms/smsForwarder.ts).
type SmsIngestReq struct {
	Sender     string    `json:"sender"     binding:"required"`
	Body       string    `json:"body"       binding:"required"`
	ReceivedAt time.Time `json:"receivedAt" binding:"required"`
	// "live" (caught on arrival) or "inbox" (imported from Settings). The same
	// message can come both ways; dedupe on sender + receivedAt.
	Source string `json:"source" binding:"omitempty,oneof=live inbox"`
}

type SmsHandler interface {
	Ingest(c *gin.Context)
}

type smsHandler struct {
	service service.SmsService
}

func NewSmsHandler(service service.SmsService) SmsHandler {
	return &smsHandler{service: service}
}

// Ingest runs the SMS through cipher's decisions endpoint and logs the
// verdict and cost (see service.SmsService) until the SMS queue exists. A
// non-2xx makes the app keep the message for retry and show the error.
func (h *smsHandler) Ingest(c *gin.Context) {
	ctx := c.Request.Context()

	var body SmsIngestReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.Ingest(ctx, service.SmsMessage{
		Sender:     body.Sender,
		Body:       body.Body,
		ReceivedAt: body.ReceivedAt,
		Source:     body.Source,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}
