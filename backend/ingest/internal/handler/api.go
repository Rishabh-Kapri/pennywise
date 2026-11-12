// Package handler is ingest's HTTP side: it turns requests into
// ports.InboundMessage and hands them to the ingest service.
package handler

import (
	"cmp"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// maxRequestBytes caps one request. Bank alerts are a few KB; this leaves
// room for HTML emails without letting a client stream megabytes in.
const maxRequestBytes = 256 << 10

// APIMessageReq is one message posted directly by a client.
type APIMessageReq struct {
	// ExternalID is the client's own ID for the message. Sending the same ID
	// again is a no-op, so retries are safe.
	ExternalID string `json:"externalId" binding:"omitempty,max=200"`
	Sender     string `json:"sender"     binding:"required,max=320"`
	Subject    string `json:"subject"    binding:"max=998"`
	Body       string `json:"body"       binding:"required"`
	// ReceivedAt is when the message arrived at the client. Required when
	// ExternalID is empty: the dedupe ID is then derived from the content
	// and this time.
	ReceivedAt *time.Time `json:"receivedAt"`
}

// SmsMessageReq is one SMS forwarded by the Android app. The app may catch a
// message live and import it from the inbox later, so anything saying how it
// arrived must stay out of the dedupe ID.
type SmsMessageReq struct {
	Sender     string    `json:"sender"     binding:"required,max=320"`
	Body       string    `json:"body"       binding:"required"`
	ReceivedAt time.Time `json:"receivedAt" binding:"required"`
}

// ConnectMailboxReq carries the one-time code from the consent screen.
type ConnectMailboxReq struct {
	Provider string `json:"provider" binding:"max=50"` // default gmail
	Code     string `json:"code"     binding:"required,max=4096"`
}

// ConnectionView is a mail connection without its credentials.
type ConnectionView struct {
	ID         uuid.UUID                  `json:"id"`
	Provider   string                     `json:"provider"`
	Account    string                     `json:"account"`
	Status     model.MailConnectionStatus `json:"status"`
	LastSyncAt *time.Time                 `json:"lastSyncAt,omitempty"`
	LastError  string                     `json:"lastError,omitempty"`
	PausedAt   *time.Time                 `json:"pausedAt,omitempty"`
}

// UpdateConnectionReq pauses or resumes a mailbox.
type UpdateConnectionReq struct {
	Paused *bool `json:"paused" binding:"required"`
}

func connectionView(conn model.MailConnection) ConnectionView {
	return ConnectionView{
		ID:         conn.ID,
		Provider:   conn.Provider,
		Account:    conn.Account,
		Status:     conn.Status,
		LastSyncAt: conn.LastSyncAt,
		LastError:  conn.LastError,
		PausedAt:   conn.PausedAt,
	}
}

// BackfillCandidate is a recent email, without its body.
type BackfillCandidate struct {
	ExternalID string    `json:"externalId"`
	Sender     string    `json:"sender"`
	Subject    string    `json:"subject"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// BackfillReq names the candidates to import.
type BackfillReq struct {
	ExternalIDs []string `json:"externalIds" binding:"required,min=1,dive,required,max=200"`
}

// TestMailboxReq names the mailbox to sync, as a provider push would.
type TestMailboxReq struct {
	Provider string `json:"provider" binding:"max=50"` // default gmail
	Account  string `json:"account"  binding:"required,max=320"`
}

// APIHandler serves /api/ingest. Every route writes to or reads the budget in
// X-Budget-ID, which the caller's middleware has already checked.
type APIHandler struct {
	ingest  service.IngestService
	mailbox service.MailboxService
}

func NewAPIHandler(ingest service.IngestService, mailbox service.MailboxService) *APIHandler {
	return &APIHandler{ingest: ingest, mailbox: mailbox}
}

// Register adds the routes, each with the API key scope it needs.
func (h *APIHandler) Register(r gin.IRoutes) {
	read := sharedMiddleware.RequireScopes(model.ScopeRead)
	write := sharedMiddleware.RequireScopes(model.ScopeWrite)
	r.POST("/messages", write, h.Ingest)
	r.POST("/sms", write, h.Sms)
	r.GET("/connections", read, h.ListConnections)
	r.POST("/connections", write, h.Connect)
	r.PATCH("/connections/:id", write, h.UpdateConnection)
	r.DELETE("/connections/:id", write, h.Disconnect)
	r.GET("/connections/:id/backfill", read, h.BackfillCandidates)
	r.POST("/connections/:id/backfill", write, h.Backfill)
	r.POST("/test", write, h.Test)
}

// Ingest queues a message posted by a client. Replies 202 with the queued jobs.
func (h *APIHandler) Ingest(c *gin.Context) {
	var req APIMessageReq
	budgetID, ok := bindForBudget(c, &req)
	if !ok {
		return
	}
	if req.ExternalID == "" && req.ReceivedAt == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "send externalId or receivedAt, so a retried request isn't counted twice",
		})
		return
	}
	receivedAt := time.Now()
	if req.ReceivedAt != nil {
		receivedAt = *req.ReceivedAt
	}
	h.accept(c, budgetID, ports.InboundMessage{
		Source:     model.SourceAPI,
		Provider:   ports.ProviderAPI,
		ExternalID: req.ExternalID,
		Sender:     req.Sender,
		Subject:    req.Subject,
		Body:       req.Body,
		ReceivedAt: receivedAt.UTC(),
	})
}

// Sms queues an SMS from the Android app. Replies 202 with the queued jobs.
func (h *APIHandler) Sms(c *gin.Context) {
	var req SmsMessageReq
	budgetID, ok := bindForBudget(c, &req)
	if !ok {
		return
	}
	h.accept(c, budgetID, ports.InboundMessage{
		Source:     model.SourceSMS,
		Provider:   ports.ProviderAndroid,
		Sender:     req.Sender,
		Body:       req.Body,
		ReceivedAt: req.ReceivedAt.UTC(),
	})
}

func (h *APIHandler) accept(c *gin.Context, budgetID uuid.UUID, msg ports.InboundMessage) {
	accepted, err := h.ingest.Accept(c.Request.Context(), []uuid.UUID{budgetID}, msg)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, accepted)
}

// Connect connects the mailbox behind a consent code to the budget.
func (h *APIHandler) Connect(c *gin.Context) {
	var req ConnectMailboxReq
	budgetID, ok := bindForBudget(c, &req)
	if !ok {
		return
	}
	provider := cmp.Or(req.Provider, ports.ProviderGmail)
	conn, err := h.mailbox.Connect(c.Request.Context(), budgetID, provider, req.Code)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, connectionView(*conn))
}

// UpdateConnection pauses or resumes a mailbox.
func (h *APIHandler) UpdateConnection(c *gin.Context) {
	budgetID, connectionID, ok := connectionTarget(c)
	if !ok {
		return
	}
	var req UpdateConnectionReq
	if !bind(c, &req) {
		return
	}
	conn, err := h.mailbox.SetPaused(c.Request.Context(), budgetID, connectionID, *req.Paused)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, connectionView(*conn))
}

// Disconnect removes a mailbox from the budget. Replies 204.
func (h *APIHandler) Disconnect(c *gin.Context) {
	budgetID, connectionID, ok := connectionTarget(c)
	if !ok {
		return
	}
	if err := h.mailbox.Disconnect(c.Request.Context(), budgetID, connectionID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// BackfillCandidates lists a mailbox's recent emails, without bodies.
func (h *APIHandler) BackfillCandidates(c *gin.Context) {
	budgetID, connectionID, ok := connectionTarget(c)
	if !ok {
		return
	}
	messages, err := h.mailbox.BackfillCandidates(c.Request.Context(), budgetID, connectionID)
	if err != nil {
		writeError(c, err)
		return
	}
	candidates := make([]BackfillCandidate, 0, len(messages))
	for _, m := range messages {
		candidates = append(candidates, BackfillCandidate{
			ExternalID: m.ExternalID,
			Sender:     m.Sender,
			Subject:    m.Subject,
			ReceivedAt: m.ReceivedAt,
		})
	}
	c.JSON(http.StatusOK, candidates)
}

// Backfill queues the import of the chosen emails. Replies 202 with the job.
func (h *APIHandler) Backfill(c *gin.Context) {
	budgetID, connectionID, ok := connectionTarget(c)
	if !ok {
		return
	}
	var req BackfillReq
	if !bind(c, &req) {
		return
	}
	queued, err := h.mailbox.QueueBackfill(c.Request.Context(), budgetID, connectionID, req.ExternalIDs)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"jobId": queued.JobID})
}

// ListConnections lists the budget's mailboxes.
func (h *APIHandler) ListConnections(c *gin.Context) {
	budgetID, ok := budget(c)
	if !ok {
		return
	}
	connections, err := h.mailbox.ListConnections(c.Request.Context(), budgetID)
	if err != nil {
		writeError(c, err)
		return
	}
	views := make([]ConnectionView, 0, len(connections))
	for _, conn := range connections {
		views = append(views, connectionView(conn))
	}
	c.JSON(http.StatusOK, views)
}

// Test queues a mailbox.changed job for one of the budget's mailboxes, as a
// provider push would. Replies 202 with the job.
func (h *APIHandler) Test(c *gin.Context) {
	var req TestMailboxReq
	budgetID, ok := bindForBudget(c, &req)
	if !ok {
		return
	}
	provider := cmp.Or(req.Provider, ports.ProviderGmail)
	res, err := h.mailbox.Test(c.Request.Context(), budgetID, provider, req.Account)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"jobId": res.JobID})
}

// The helpers below answer the request themselves when they return false.

// bind reads the JSON body into req, at most maxRequestBytes of it.
func bind(c *gin.Context, req any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	return true
}

// budget reads the budget BudgetIdMiddleware put on the request.
func budget(c *gin.Context) (uuid.UUID, bool) {
	budgetID, err := utils.BudgetIDFromContext(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing budget"})
		return uuid.Nil, false
	}
	return budgetID, true
}

func bindForBudget(c *gin.Context, req any) (uuid.UUID, bool) {
	if !bind(c, req) {
		return uuid.Nil, false
	}
	return budget(c)
}

// connectionTarget reads the budget and connection a /connections/:id route is for.
func connectionTarget(c *gin.Context) (budgetID, connectionID uuid.UUID, ok bool) {
	if budgetID, ok = budget(c); !ok {
		return uuid.Nil, uuid.Nil, false
	}
	connectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return uuid.Nil, uuid.Nil, false
	}
	return budgetID, connectionID, true
}

// writeError maps service errors to HTTP statuses.
func writeError(c *gin.Context, err error) {
	switch {
	case errs.HasCode(err, errs.CodeIngestInvalidMessage), errs.HasCode(err, errs.CodeJobInvalidOption):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errs.HasCode(err, errs.CodeMailNoRefreshToken):
		c.JSON(http.StatusConflict, gin.H{"error": "google didn't return a long-lived token"})
	case errs.HasCode(err, errs.CodeMailAuthFailed):
		c.JSON(http.StatusBadRequest, gin.H{"error": "google refused this sign-in code"})
	case errs.HasCode(err, errs.CodeMailRequestFailed):
		logger.Logger(c.Request.Context()).Error("mail provider request failed", "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "the mail provider refused the request"})
	case errs.HasCode(err, errs.CodeMailNotConfigured), errs.HasCode(err, errs.CodeAdapterNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mail connections are not set up"})
	case errs.HasCode(err, errs.CodeConnectionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such mailbox on this budget"})
	case errs.HasCode(err, errs.CodeIngestQueueUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ingestion is not available right now"})
	default:
		logger.Logger(c.Request.Context()).Error("ingest failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not accept message"})
	}
}
