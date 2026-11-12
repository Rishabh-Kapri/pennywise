package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type TransactionHandler interface {
	List(c *gin.Context)
	ListNormalized(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	UpdateLocation(c *gin.Context)
	UpdateStatus(c *gin.Context)
	BackfillLearning(c *gin.Context)
	LearningStatus(c *gin.Context)
	// DeleteById deletes a transaction by its ID.
	// It retrieves the budget context and the transaction ID from the request parameters,
	// parses the ID, and then calls the service to perform the deletion.
	// Returns appropriate HTTP status and message based on the outcome.
	DeleteById(c *gin.Context)
	// BulkDelete is POST /transactions/bulk-delete.
	BulkDelete(c *gin.Context)
}

type transactionHandler struct {
	service service.TransactionService
}

func NewTransactionHandler(service service.TransactionService) TransactionHandler {
	return &transactionHandler{service: service}
}

// transactionIDURI is the :id path param of the transaction routes.
type transactionIDURI struct {
	ID string `uri:"id" binding:"required,uuid"`
}

// bindTransactionID reads :id, or answers 400 and returns false.
func bindTransactionID(c *gin.Context) (uuid.UUID, bool) {
	var uri transactionIDURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction id"})
		return uuid.Nil, false
	}
	// The uuid rule already checked the format.
	return uuid.MustParse(uri.ID), true
}

// listTransactionsQuery is the query string of ListNormalized. Each ID
// filter comes either repeated (accountId[]=a&accountId[]=b) or as one
// comma-joined value (accountId=a,b).
type listTransactionsQuery struct {
	AccountID   string   `form:"accountId"`
	AccountIDs  []string `form:"accountId[]"`
	CategoryID  string   `form:"categoryId"`
	CategoryIDs []string `form:"categoryId[]"`
	PayeeID     string   `form:"payeeId"`
	PayeeIDs    []string `form:"payeeId[]"`
	TagID       string   `form:"tagId"`
	TagIDs      []string `form:"tagId[]"`

	StartDate string `form:"startDate" binding:"omitempty,datetime=2006-01-02"`
	EndDate   string `form:"endDate"   binding:"omitempty,datetime=2006-01-02"`
	Note      string `form:"note"`
	Status    string `form:"status" binding:"omitempty,oneof=UNAPPROVED APPROVED REJECTED MANUAL"`

	Limit     uint64 `form:"limit,default=30"`
	GroupBy   string `form:"groupBy,default=month"`
	SortOrder string `form:"sortOrder,default=DESC" binding:"oneof=ASC DESC asc desc"`
	Cursor    string `form:"cursor"`
}

// idParam merges the two ways an ID filter can arrive.
func idParam(repeated []string, joined string) []string {
	if len(repeated) == 0 && strings.TrimSpace(joined) != "" {
		return []string{joined}
	}
	return repeated
}

// bindingError turns a binding failure into a short message naming the
// field, instead of the validator's "Key: 'X.Y' Error:Field validation ...".
func bindingError(err error) string {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) && len(verrs) > 0 {
		f := verrs[0].Field()
		return "invalid " + strings.ToLower(f[:1]) + f[1:]
	}
	return err.Error()
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

	// clients send either repeated params or a single comma-joined value,
	// so split every element before parsing
	for _, joined := range ids {
		for _, id := range strings.Split(joined, ",") {
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
	}

	return filterIds, nil
}

func (h *transactionHandler) ListNormalized(c *gin.Context) {
	ctx := c.Request.Context()

	var q listTransactionsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindingError(err)})
		return
	}

	accountIds := idParam(q.AccountIDs, q.AccountID)
	categoryIds := idParam(q.CategoryIDs, q.CategoryID)
	payeeIds := idParam(q.PayeeIDs, q.PayeeID)
	tagIds := idParam(q.TagIDs, q.TagID)

	txnFilter := model.TransactionFilter{
		Limit:        q.Limit,
		GroupBy:      &q.GroupBy,
		SortOrder:    q.SortOrder,
		CursorString: q.Cursor,
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

	if len(tagIds) > 0 {
		ids, err := stringToUUIDs(tagIds)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing tagId"})
			return
		}
		txnFilter.TagIDs = ids
	}

	if note := strings.TrimSpace(q.Note); note != "" {
		txnFilter.Note = &note
	}
	if q.StartDate != "" {
		txnFilter.StartDate = &q.StartDate
	}
	if q.EndDate != "" {
		txnFilter.EndDate = &q.EndDate
	}
	if q.Status != "" {
		status := model.TransactionStatus(q.Status)
		txnFilter.Status = &status
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	parsedId, ok := bindTransactionID(c)
	if !ok {
		return
	}

	var body model.Transaction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.Update(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, body)
}

func (h *transactionHandler) UpdateLocation(c *gin.Context) {
	ctx := c.Request.Context()
	parsedId, ok := bindTransactionID(c)
	if !ok {
		return
	}

	var body model.TransactionLocationReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.service.UpdateLocation(ctx, parsedId, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *transactionHandler) UpdateStatus(c *gin.Context) {
	ctx := c.Request.Context()
	parsedId, ok := bindTransactionID(c)
	if !ok {
		return
	}

	var body model.TransactionStatusReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.service.UpdateStatus(ctx, parsedId, body.Status)
	if err != nil {
		if errs.HasCode(err, errs.CodeTransactionInvalidStatus) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": errs.CodeTransactionInvalidStatus})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}

// LearningStatus reports how many of the budget's predictions have taught the
// pipeline, so the UI can show whether a backfill is worth running.
func (h *transactionHandler) LearningStatus(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := h.service.LearningStatus(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// BackfillLearning kicks off learning over past predictions that never taught
// the pipeline. Returns as soon as the work is queued -- it runs in the
// background because every item costs two LLM round-trips and a page of them
// runs well past the proxy's request timeout. Poll LearningStatus for progress.
func (h *transactionHandler) BackfillLearning(c *gin.Context) {
	ctx := c.Request.Context()

	var body model.LearningBackfillRequest
	// An empty body is the common case: backfill the next page with defaults.
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	result, err := h.service.BackfillLearning(ctx, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if result.Started {
		c.JSON(http.StatusAccepted, result)
		return
	}
	// Nothing queued: either a run is already in flight or there is nothing
	// left to learn. Both are fine, neither started work.
	c.JSON(http.StatusOK, result)
}

// maxBulkDelete bounds one request, which runs in a single database transaction.
const maxBulkDelete = 500

type bulkDeleteRequest struct {
	IDs []uuid.UUID `json:"ids" binding:"required,min=1,dive,required"`
}

func (h *transactionHandler) BulkDelete(c *gin.Context) {
	var body bulkDeleteRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindingError(err)})
		return
	}
	if len(body.IDs) > maxBulkDelete {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("at most %d transactions at a time", maxBulkDelete)})
		return
	}

	deleted, err := h.service.BulkDelete(c.Request.Context(), body.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

func (h *transactionHandler) DeleteById(c *gin.Context) {
	ctx := c.Request.Context()

	parsedId, ok := bindTransactionID(c)
	if !ok {
		return
	}

	err := h.service.DeleteById(ctx, parsedId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, nil)
}
