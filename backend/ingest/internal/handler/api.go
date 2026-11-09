// Package handler is ingest's HTTP side: it turns requests into
// ports.InboundMessage and hands them to the ingest service.
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// maxRequestBytes caps one request. Bank alerts are a few KB; this leaves
// room for HTML emails without letting a client stream megabytes in.
const maxRequestBytes = 256 << 10

// APIProvider is the provider name for messages posted to the API.
const APIProvider = "api"

// SmsProvider is the provider name for SMS forwarded by the Android app.
const SmsProvider = "android"

// defaultMailProvider is used when a request names no provider.
const defaultMailProvider = "gmail"

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
	ExternalIDs []string `json:"externalIds" binding:"required,min=1,max=500,dive,required,max=200"`
}

// TestMailboxReq names the mailbox to sync, as a provider push would.
type TestMailboxReq struct {
	Provider string `json:"provider" binding:"max=50"` // default gmail
	Account  string `json:"account"  binding:"required,max=320"`
}

type APIHandler interface {
	// Ingest is POST /api/ingest/messages. The budget comes from
	// X-Budget-ID (BudgetIdMiddleware). Replies 202 with the queued jobs.
	Ingest(c *gin.Context)
	// Sms is POST /api/ingest/sms, for SMS from the Android app. The budget
	// comes from X-Budget-ID. Replies 202 with the queued jobs.
	Sms(c *gin.Context)
	// Connect is POST /api/ingest/connections: it connects the mailbox behind a
	// consent code to the budget in X-Budget-ID.
	Connect(c *gin.Context)
	// BackfillCandidates is GET /api/ingest/connections/:id/backfill.
	BackfillCandidates(c *gin.Context)
	// Backfill is POST /api/ingest/connections/:id/backfill. It queues the
	// import and replies 202 with the job.
	Backfill(c *gin.Context)
	// UpdateConnection is PATCH /api/ingest/connections/:id: it pauses or
	// resumes a mailbox.
	UpdateConnection(c *gin.Context)
	// Disconnect is DELETE /api/ingest/connections/:id: it removes a mailbox
	// from the budget in X-Budget-ID. Replies 204.
	Disconnect(c *gin.Context)
	// ListConnections is GET /api/ingest/connections.
	ListConnections(c *gin.Context)
	// Test is POST /api/ingest/test: it queues a mailbox.changed job for a
	// mailbox of the budget in X-Budget-ID. Replies 202 with the job.
	Test(c *gin.Context)
}

type apiHandler struct {
	ingest  service.IngestService
	mailbox service.MailboxService
}

func NewAPIHandler(ingest service.IngestService, mailbox service.MailboxService) APIHandler {
	return &apiHandler{ingest: ingest, mailbox: mailbox}
}

func (h *apiHandler) Ingest(c *gin.Context) {
	ctx := c.Request.Context()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)

	var req APIMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ExternalID == "" && req.ReceivedAt == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "send externalId or receivedAt, so a retried request isn't counted twice",
		})
		return
	}

	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing budget"})
		return
	}

	receivedAt := time.Now().UTC()
	if req.ReceivedAt != nil {
		receivedAt = req.ReceivedAt.UTC()
	}
	externalID := req.ExternalID
	if externalID == "" {
		externalID = contentID(req.Sender, req.Subject, req.Body, receivedAt)
	}

	queued, err := h.ingest.Accept(ctx, ports.InboundMessage{
		Source:     ports.ApiSource,
		Provider:   APIProvider,
		BudgetID:   []uuid.UUID{budgetID},
		ExternalID: externalID,
		Sender:     req.Sender,
		Subject:    req.Subject,
		Body:       req.Body,
		ReceivedAt: receivedAt,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"externalId": externalID, "jobs": queued})
}

func (h *apiHandler) Sms(c *gin.Context) {
	ctx := c.Request.Context()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)

	var req SmsMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing budget"})
		return
	}

	receivedAt := req.ReceivedAt.UTC()
	externalID := contentID(req.Sender, "", req.Body, receivedAt)
	queued, err := h.ingest.Accept(ctx, ports.InboundMessage{
		Source:     ports.SmsSource,
		Provider:   SmsProvider,
		BudgetID:   []uuid.UUID{budgetID},
		ExternalID: externalID,
		Sender:     req.Sender,
		Body:       req.Body,
		ReceivedAt: receivedAt,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"externalId": externalID, "jobs": queued})
}

func (h *apiHandler) Connect(c *gin.Context) {
	ctx := c.Request.Context()
	var req ConnectMailboxReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing budget"})
		return
	}
	provider := req.Provider
	if provider == "" {
		provider = defaultMailProvider
	}

	conn, err := h.mailbox.Connect(ctx, budgetID, provider, req.Code)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, connectionView(*conn))
}

func (h *apiHandler) UpdateConnection(c *gin.Context) {
	budgetID, connectionID, ok := connectionTarget(c)
	if !ok {
		return
	}
	var req UpdateConnectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	conn, err := h.mailbox.SetPaused(c.Request.Context(), budgetID, connectionID, *req.Paused)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, connectionView(*conn))
}

func (h *apiHandler) Disconnect(c *gin.Context) {
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

// connectionTarget reads the budget and connection a /connections/:id route is for. On
// false it has already answered the request.
func connectionTarget(c *gin.Context) (budgetID, connectionID uuid.UUID, ok bool) {
	budgetID, err := utils.BudgetIDFromContext(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing budget"})
		return uuid.Nil, uuid.Nil, false
	}
	connectionID, err = uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return uuid.Nil, uuid.Nil, false
	}
	return budgetID, connectionID, true
}

func (h *apiHandler) BackfillCandidates(c *gin.Context) {
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

func (h *apiHandler) Backfill(c *gin.Context) {
	budgetID, connectionID, ok := connectionTarget(c)
	if !ok {
		return
	}
	var req BackfillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	queued, err := h.mailbox.QueueBackfill(c.Request.Context(), budgetID, connectionID, req.ExternalIDs)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"jobId": queued.JobID})
}

func (h *apiHandler) ListConnections(c *gin.Context) {
	ctx := c.Request.Context()
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing budget"})
		return
	}
	connections, err := h.mailbox.ListConnections(ctx, budgetID)
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

func (h *apiHandler) Test(c *gin.Context) {
	ctx := c.Request.Context()
	var req TestMailboxReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing budget"})
		return
	}
	provider := req.Provider
	if provider == "" {
		provider = defaultMailProvider
	}

	res, err := h.mailbox.Test(ctx, budgetID, provider, req.Account)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"jobId": res.JobID})
}

// contentID derives a stable dedupe ID: the same message sent twice gets the
// same ID, while two identical alerts at different times don't collide.
func contentID(sender, subject, body string, receivedAt time.Time) string {
	sum := sha256.Sum256([]byte(sender + "\x00" + subject + "\x00" + body + "\x00" +
		receivedAt.Format(time.RFC3339Nano)))
	return "sha256:" + hex.EncodeToString(sum[:])
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
