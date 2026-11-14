package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BudgetSharingHandler serves the budget's members and invites
// (/budget-members, /budget-invites: budget-scoped) and the invitee's side
// (/invites, /budgets/:id/leave: user-scoped, because they aren't a member).
type BudgetSharingHandler interface {
	ListMembers(c *gin.Context)
	UpdateMember(c *gin.Context)
	RemoveMember(c *gin.Context)

	ListInvites(c *gin.Context)
	CreateInvite(c *gin.Context)
	RevokeInvite(c *gin.Context)

	ListMyInvites(c *gin.Context)
	AcceptInvite(c *gin.Context)
	DeclineInvite(c *gin.Context)
	Leave(c *gin.Context)
}

type budgetSharingHandler struct {
	service service.BudgetSharingService
}

func NewBudgetSharingHandler(service service.BudgetSharingService) BudgetSharingHandler {
	return &budgetSharingHandler{service: service}
}

// uuidParam parses a UUID path parameter, answering 400 when it isn't one.
func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return uuid.Nil, false
	}
	return id, true
}

// currentUser reads the authenticated user, answering 401 without one.
func currentUser(c *gin.Context) (uuid.UUID, bool) {
	userID, err := utils.UserIDFromContext(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return uuid.Nil, false
	}
	return userID, true
}

func (h *budgetSharingHandler) ListMembers(c *gin.Context) {
	members, err := h.service.Members(c.Request.Context())
	if err != nil {
		writeStatusError(c, err)
		return
	}
	c.JSON(http.StatusOK, members)
}

func (h *budgetSharingHandler) UpdateMember(c *gin.Context) {
	userID, ok := uuidParam(c, "userId")
	if !ok {
		return
	}
	var body model.UpdateBudgetMemberRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.ChangeRole(c.Request.Context(), userID, body.Role); err != nil {
		writeStatusError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *budgetSharingHandler) RemoveMember(c *gin.Context) {
	userID, ok := uuidParam(c, "userId")
	if !ok {
		return
	}
	if err := h.service.RemoveMember(c.Request.Context(), userID); err != nil {
		writeStatusError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *budgetSharingHandler) ListInvites(c *gin.Context) {
	invites, err := h.service.Invites(c.Request.Context())
	if err != nil {
		writeStatusError(c, err)
		return
	}
	c.JSON(http.StatusOK, invites)
}

func (h *budgetSharingHandler) CreateInvite(c *gin.Context) {
	var body model.CreateBudgetInviteRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	invite, err := h.service.Invite(c.Request.Context(), body)
	if err != nil {
		writeStatusError(c, err)
		return
	}
	c.JSON(http.StatusCreated, invite)
}

func (h *budgetSharingHandler) RevokeInvite(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.RevokeInvite(c.Request.Context(), id); err != nil {
		writeStatusError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *budgetSharingHandler) ListMyInvites(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	invites, err := h.service.MyInvites(c.Request.Context(), userID)
	if err != nil {
		writeStatusError(c, err)
		return
	}
	c.JSON(http.StatusOK, invites)
}

func (h *budgetSharingHandler) AcceptInvite(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	budgetID, err := h.service.AcceptInvite(c.Request.Context(), userID, id)
	if err != nil {
		writeStatusError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"budgetId": budgetID})
}

func (h *budgetSharingHandler) DeclineInvite(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeclineInvite(c.Request.Context(), userID, id); err != nil {
		writeStatusError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *budgetSharingHandler) Leave(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	budgetID, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Leave(c.Request.Context(), userID, budgetID); err != nil {
		writeStatusError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
