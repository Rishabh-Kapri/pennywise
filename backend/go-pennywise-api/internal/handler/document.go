package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DocumentHandler interface {
	Upload(c *gin.Context)
	ListByTransaction(c *gin.Context)
	Content(c *gin.Context)
	Delete(c *gin.Context)
}

type documentHandler struct {
	service service.DocumentService
}

func NewDocumentHandler(service service.DocumentService) DocumentHandler {
	return &documentHandler{service: service}
}

func (h *documentHandler) Upload(c *gin.Context) {
	ctx := c.Request.Context()

	transactionId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing id"})
		return
	}

	// bound the whole request body before touching the multipart reader
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxDocumentSizeBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if _, tooLarge := err.(*http.MaxBytesError); tooLarge {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("file exceeds the %dMB limit", service.MaxDocumentSizeBytes>>20),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart 'file' field is required"})
		return
	}
	defer file.Close()

	doc, err := h.service.Upload(ctx, transactionId, header.Filename, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h *documentHandler) ListByTransaction(c *gin.Context) {
	ctx := c.Request.Context()

	transactionId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing id"})
		return
	}

	docs, err := h.service.ListByTransaction(ctx, transactionId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, docs)
}

func (h *documentHandler) Content(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := uuid.Parse(c.Param("docId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing docId"})
		return
	}

	doc, reader, err := h.service.Content(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	defer reader.Close()

	disposition := "inline"
	if c.Query("download") == "true" {
		disposition = "attachment"
	}
	c.Header("Content-Type", doc.MimeType)
	c.Header("Content-Length", strconv.FormatInt(doc.SizeBytes, 10))
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, doc.FileName))
	c.Header("Cache-Control", "private, max-age=3600")

	http.ServeContent(c.Writer, c.Request, doc.FileName, doc.UpdatedAt, reader)
}

func (h *documentHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := uuid.Parse(c.Param("docId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error while parsing docId"})
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nil)
}
