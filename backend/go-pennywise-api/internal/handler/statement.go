package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type StatementHandler interface {
	Upload(c *gin.Context)
	List(c *gin.Context)
	Get(c *gin.Context)
	SetSectionAccount(c *gin.Context)
	CreateMissing(c *gin.Context)
	Retry(c *gin.Context)
	Delete(c *gin.Context)
}

type statementHandler struct {
	service service.StatementService
}

func NewStatementHandler(service service.StatementService) StatementHandler {
	return &statementHandler{service: service}
}

// Upload takes one PDF (`file`), an optional default `accountId` and an
// optional `password`, and answers 202 while extraction runs. A file already
// uploaded answers 200 with the existing import.
func (h *statementHandler) Upload(c *gin.Context) {
	ctx := c.Request.Context()

	// headroom over the file cap for the other form fields and multipart framing
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxStatementSizeBytes+64<<10)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("statement exceeds the %dMB limit", service.MaxStatementSizeBytes>>20),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart 'file' field is required"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, service.MaxStatementSizeBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read the uploaded file"})
		return
	}
	if len(data) > service.MaxStatementSizeBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("statement exceeds the %dMB limit", service.MaxStatementSizeBytes>>20),
		})
		return
	}

	var accountID *uuid.UUID
	if raw := strings.TrimSpace(c.PostForm("accountId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accountId"})
			return
		}
		accountID = &id
	}

	imp, duplicate, err := h.service.Upload(ctx, header.Filename, data, c.PostForm("password"), accountID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrStatementPasswordRequired):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "password_required"})
		case errors.Is(err, service.ErrStatementNoText):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "no_text"})
		default:
			writeStatementError(c, err)
		}
		return
	}
	if duplicate {
		c.JSON(http.StatusOK, gin.H{"import": imp, "duplicate": true})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"import": imp, "duplicate": false})
}

func (h *statementHandler) List(c *gin.Context) {
	imports, err := h.service.List(c.Request.Context())
	if err != nil {
		writeStatementError(c, err)
		return
	}
	c.JSON(http.StatusOK, imports)
}

func (h *statementHandler) Get(c *gin.Context) {
	id, ok := statementID(c)
	if !ok {
		return
	}
	rec, err := h.service.Reconcile(c.Request.Context(), id)
	if err != nil {
		writeStatementError(c, err)
		return
	}
	c.JSON(http.StatusOK, rec)
}

func (h *statementHandler) SetSectionAccount(c *gin.Context) {
	id, ok := statementID(c)
	if !ok {
		return
	}
	section, err := strconv.Atoi(c.Param("section"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid section"})
		return
	}
	var req model.StatementSectionAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rec, err := h.service.SetSectionAccount(c.Request.Context(), id, section, req.AccountID)
	if err != nil {
		writeStatementError(c, err)
		return
	}
	c.JSON(http.StatusOK, rec)
}

func (h *statementHandler) CreateMissing(c *gin.Context) {
	id, ok := statementID(c)
	if !ok {
		return
	}
	var req model.StatementCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.CreateMissing(c.Request.Context(), id, req)
	if err != nil {
		writeStatementError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *statementHandler) Retry(c *gin.Context) {
	id, ok := statementID(c)
	if !ok {
		return
	}
	imp, err := h.service.Retry(c.Request.Context(), id)
	if err != nil {
		writeStatementError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, imp)
}

func (h *statementHandler) Delete(c *gin.Context) {
	id, ok := statementID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		writeStatementError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func statementID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid statement id"})
		return uuid.Nil, false
	}
	return id, true
}

func writeStatementError(c *gin.Context, err error) {
	var e *errs.Error
	switch {
	case errors.Is(err, service.ErrStatementNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.As(err, &e) && e.Code == errs.CodeInvalidArgument:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		logger.Logger(c.Request.Context()).Error("statement request failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
