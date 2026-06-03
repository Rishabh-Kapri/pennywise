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
	UpdateStatus(c *gin.Context)
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

func stringToUUIDs(ids []string) ([]uuid.UUID, error) {
	var filterIds []uuid.UUID

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		parsedId, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		filterIds = append(filterIds, parsedId)
	}

	return filterIds, nil
}

func (h *transactionHandler) ListNormalized(c *gin.Context) {
	ctx := c.Request.Context()

	accountIdParam := strings.TrimSpace(c.Query("accountId"))
	categoryIdParam := strings.TrimSpace(c.Query("categoryId"))
	payeeIdParam := strings.TrimSpace(c.Query("payeeId"))
	startDateParam := strings.TrimSpace(c.DefaultQuery("startDate", ""))
	endDateParam := strings.TrimSpace(c.DefaultQuery("endDate", ""))
	noteParam := strings.TrimSpace(c.DefaultQuery("note", ""))

	limit := c.DefaultQuery("limit", "30")
	limitInt, err := strconv.ParseUint(limit, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing limit"})
		return
	}

	groupBy := c.DefaultQuery("groupBy", "month")
	sortOrder := c.DefaultQuery("sortOrder", "DESC")
	cursor := c.Query("cursor")

	accountIds := c.QueryArray("accountId[]")
	if len(accountIds) == 0 && accountIdParam != "" {
		accountIds = strings.Split(accountIdParam, ",")
	}

	categoryIds := c.QueryArray("categoryId[]")
	if len(categoryIds) == 0 && categoryIdParam != "" {
		categoryIds = strings.Split(categoryIdParam, ",")
	}

	payeeIds := c.QueryArray("payeeId[]")
	if len(payeeIds) == 0 && payeeIdParam != "" {
		payeeIds = strings.Split(payeeIdParam, ",")
	}

	logger.Logger(ctx).Info("listing normalized transactions", "accountIdParam", accountIdParam)

	txnFilter := model.TransactionFilter{
		Limit:        limitInt,
		GroupBy:      &groupBy,
		SortOrder:    sortOrder,
		CursorString: cursor,
	}

	if len(accountIds) > 0 {
		ids, err := stringToUUIDs(accountIds)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing accountId"})
			return
		}
		txnFilter.AccountIDs = ids
	}

	if len(categoryIds) > 0 {
		ids, err := stringToUUIDs(categoryIds)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing categoryId"})
			return
		}
		txnFilter.CategoryIDs = ids
	}

	if len(payeeIds) > 0 {
		ids, err := stringToUUIDs(payeeIds)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing payeeId"})
			return
		}
		txnFilter.PayeeIDs = ids
	}

	if noteParam != "" {
		txnFilter.Note = &noteParam
	}

	if startDateParam != "" {
		txnFilter.StartDate = &startDateParam
	}

	if endDateParam != "" {
		txnFilter.EndDate = &endDateParam
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

func (h *transactionHandler) UpdateStatus(c *gin.Context) {
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

	var body model.TransactionStatusReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err = h.service.UpdateStatus(ctx, parsedId, body.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
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
