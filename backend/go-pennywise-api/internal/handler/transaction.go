package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TransactionHandler interface {
	List(c *gin.Context)
	ListNormalized(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	// DeleteById deletes a transaction by its ID.
	// It retrieves the budget context and the transaction ID from the request parameters,
	// parses the ID, and then calls the service to perform the deletion.
	// Returns appropriate HTTP status and message based on the outcome.
	DeleteById(c *gin.Context)
}

type transactionHandler struct {
	service service.TransactionService
}

func NewTransactionHandler(service service.TransactionService) TransactionHandler {
	return &transactionHandler{service: service}
}

func (h *transactionHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	transactions, err := h.service.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

func (h *transactionHandler) ListNormalized(c *gin.Context) {
	ctx := c.Request.Context()
	accountIdParam := strings.TrimSpace(c.Query("accountId"))

	limit := c.DefaultQuery("limit", "30")
	limitInt, err := strconv.ParseUint(limit, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing limit"})
		return
	}
	groupBy := c.DefaultQuery("groupBy", "month")
	sortOrder := c.DefaultQuery("sortOrder", "DESC")
	accountIds := c.QueryArray("accountId[]")
	cursor := c.Query("cursor")
	if len(accountIds) == 0 && accountIdParam != "" {
		accountIds = strings.Split(accountIdParam, ",")
	}

	logger.Logger(ctx).Info("listing normalized transactions", "accountIdParam", accountIdParam)
	var accountIdsFilter []uuid.UUID
	if len(accountIds) > 0 {
		for _, id := range accountIds {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			parsedId, err := uuid.Parse(id)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing accountId"})
				return
			}
			accountIdsFilter = append(accountIdsFilter, parsedId)
		}
	}

	txnFilter := model.TransactionFilter{
		AccountIDs:   accountIdsFilter,
		Limit:        limitInt,
		GroupBy:      &groupBy,
		SortOrder:    sortOrder,
		CursorString: cursor,
	}

	transactions, err := h.service.GetAllNormalized(ctx, &txnFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

func (h *transactionHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.Transaction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	createdTxns, err := h.service.Create(ctx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, createdTxns)
}

func (h *transactionHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}
	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while parsing id"})
	}

	var body model.Transaction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = h.service.Update(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, body)
}

func (h *transactionHandler) DeleteById(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := c.Params.Get("id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is needed"})
		return
	}

	parsedId, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while parsing id"})
	}

	err = h.service.DeleteById(ctx, parsedId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, nil)
}
