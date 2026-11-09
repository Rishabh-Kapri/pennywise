package handler

import (
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
)

type OnboardingHandler interface {
	Get(c *gin.Context)
	Update(c *gin.Context)
	BackfillEmails(c *gin.Context)
}

type onboardingHandler struct {
	service service.OnboardingService
}

func NewOnboardingHandler(service service.OnboardingService) OnboardingHandler {
	return &onboardingHandler{service: service}
}

func (h *onboardingHandler) Get(c *gin.Context) {
	onboarding, err := h.service.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, onboarding)
}

func (h *onboardingHandler) Update(c *gin.Context) {
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

func (h *onboardingHandler) BackfillEmails(c *gin.Context) {
	ctx := c.Request.Context()
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
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
