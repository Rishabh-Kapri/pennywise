package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
)

type OnboardingHandler struct {
	service service.OnboardingService
}

func NewOnboardingHandler(service service.OnboardingService) *OnboardingHandler {
	return &OnboardingHandler{service: service}
}

func (h *OnboardingHandler) Get(c *gin.Context) {
	onboarding, err := h.service.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, onboarding)
}

func (h *OnboardingHandler) Update(c *gin.Context) {
	var body model.UpdateBudgetOnboardingRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	onboarding, err := h.service.Update(c.Request.Context(), body)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidOnboardingStep) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, onboarding)
}

func (h *OnboardingHandler) BackfillEmails(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	var body model.OnboardingEmailBackfillRequest
	// The body is optional: no body means the default window.
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	res, err := h.service.BackfillEmails(ctx, userID, body.Days)
	switch {
	case errors.Is(err, service.ErrNoAIProvider):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "ai_provider_required"})
	case errors.Is(err, service.ErrNoGmailConnected):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "gmail_not_connected"})
	case err != nil:
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusAccepted, res)
	}
}

func (h *OnboardingHandler) Routes(r gin.IRouter) {
	r.GET("/onboarding", h.Get)
	r.PATCH("/onboarding", h.Update)
	r.POST("/onboarding/email-backfill", h.BackfillEmails)
}
