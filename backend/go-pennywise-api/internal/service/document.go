package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/storage"
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// MaxDocumentSizeBytes bounds a single receipt/document upload.
const MaxDocumentSizeBytes int64 = 10 << 20 // 10 MiB

// allowedDocumentMimeTypes maps accepted content types (detected by sniffing,
// not the client-supplied header) to the extension stored on disk.
var allowedDocumentMimeTypes = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"image/heic":      ".heic",
	"application/pdf": ".pdf",
}

type DocumentService interface {
	Upload(ctx context.Context, transactionId uuid.UUID, fileName string, body io.Reader) (*model.TransactionDocument, error)
	// UploadScan assembles captured pages into a single PDF stored as one document.
	UploadScan(ctx context.Context, transactionId uuid.UUID, pages []ScanPage) (*model.TransactionDocument, error)
	ListByTransaction(ctx context.Context, transactionId uuid.UUID) ([]model.TransactionDocument, error)
	// Content returns the document metadata and a reader over its bytes.
	Content(ctx context.Context, id uuid.UUID) (*model.TransactionDocument, io.ReadSeekCloser, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type documentService struct {
	repo    repository.TransactionDocumentRepository
	txnRepo repository.TransactionRepository
	store   storage.Store
}

func NewDocumentService(
	repo repository.TransactionDocumentRepository,
	txnRepo repository.TransactionRepository,
	store storage.Store,
) DocumentService {
	return &documentService{repo: repo, txnRepo: txnRepo, store: store}
}

// detectMimeType sniffs the leading bytes; extension is never trusted.
// HEIC is special-cased because net/http sniffs it as application/octet-stream.
func detectMimeType(head []byte, fileName string) string {
	sniffed := http.DetectContentType(head)
	if sniffed == "application/octet-stream" &&
		len(head) > 11 && string(head[4:8]) == "ftyp" &&
		strings.EqualFold(filepath.Ext(fileName), ".heic") {
		return "image/heic"
	}
	// strip parameters like "; charset=utf-8"
	if i := strings.Index(sniffed, ";"); i >= 0 {
		sniffed = strings.TrimSpace(sniffed[:i])
	}
	return sniffed
}

func (s *documentService) Upload(
	ctx context.Context,
	transactionId uuid.UUID,
	fileName string,
	body io.Reader,
) (*model.TransactionDocument, error) {
	budgetId := utils.MustBudgetID(ctx)

	// ensure the transaction exists in this budget before accepting bytes
	if _, err := s.txnRepo.GetById(ctx, budgetId, transactionId); err != nil {
		return nil, errs.Wrap(errs.CodeTransactionLookupFailed, "transaction not found", err)
	}

	head := make([]byte, 512)
	n, err := io.ReadFull(body, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, errs.Wrap(errs.CodeInternalError, "error reading upload", err)
	}
	head = head[:n]
	if n == 0 {
		return nil, errs.New(errs.CodeInvalidArgument, "uploaded file is empty")
	}

	mimeType := detectMimeType(head, fileName)
	ext, ok := allowedDocumentMimeTypes[mimeType]
	if !ok {
		return nil, errs.New(
			errs.CodeInvalidArgument,
			"unsupported file type %q; allowed: jpeg, png, webp, heic, pdf",
			mimeType,
		)
	}

	docId := uuid.New()
	relPath, err := s.store.Save(
		budgetId.String(),
		docId.String()+ext,
		io.MultiReader(strings.NewReader(string(head)), body),
	)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error storing file", err)
	}

	if fileName == "" {
		fileName = docId.String() + ext
	}

	sizeBytes, err := s.storedSize(relPath)
	if err != nil {
		s.cleanupStored(ctx, relPath)
		return nil, errs.Wrap(errs.CodeInternalError, "error verifying stored file", err)
	}

	doc, err := s.repo.Create(ctx, model.TransactionDocument{
		ID:            docId,
		BudgetID:      budgetId,
		TransactionID: transactionId,
		FileName:      fileName,
		MimeType:      mimeType,
		SizeBytes:     sizeBytes,
		StoragePath:   relPath,
	})
	if err != nil {
		s.cleanupStored(ctx, relPath)
		return nil, errs.Wrap(errs.CodeInternalError, "error saving document record", err)
	}
	return doc, nil
}

// UploadScan merges a multi-page capture into one PDF so a stack of bills
// lands as a single receipt rather than N loose images.
func (s *documentService) UploadScan(
	ctx context.Context,
	transactionId uuid.UUID,
	pages []ScanPage,
) (*model.TransactionDocument, error) {
	budgetId := utils.MustBudgetID(ctx)

	if len(pages) == 0 {
		return nil, errs.New(errs.CodeInvalidArgument, "no pages were uploaded")
	}
	if len(pages) > MaxScanPages {
		return nil, errs.New(errs.CodeInvalidArgument, "a scan can have at most %d pages", MaxScanPages)
	}
	if _, err := s.txnRepo.GetById(ctx, budgetId, transactionId); err != nil {
		return nil, errs.Wrap(errs.CodeTransactionLookupFailed, "transaction not found", err)
	}

	// sniff every page before spending time on assembly
	for i, page := range pages {
		head := page.Data
		if len(head) > 512 {
			head = head[:512]
		}
		mimeType := detectMimeType(head, page.Name)
		if mimeType != "image/jpeg" && mimeType != "image/png" {
			return nil, errs.New(
				errs.CodeInvalidArgument,
				"page %d has unsupported type %q; scans must be jpeg or png",
				i+1,
				mimeType,
			)
		}
	}

	pdfBytes, err := buildScannedPDF(pages)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInvalidArgument, "error assembling scan", err)
	}

	docId := uuid.New()
	relPath, err := s.store.Save(budgetId.String(), docId.String()+".pdf", bytes.NewReader(pdfBytes))
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error storing scan", err)
	}

	doc, err := s.repo.Create(ctx, model.TransactionDocument{
		ID:            docId,
		BudgetID:      budgetId,
		TransactionID: transactionId,
		FileName:      fmt.Sprintf("scan-%s.pdf", time.Now().Format("20060102-150405")),
		MimeType:      "application/pdf",
		SizeBytes:     int64(len(pdfBytes)),
		StoragePath:   relPath,
	})
	if err != nil {
		s.cleanupStored(ctx, relPath)
		return nil, errs.Wrap(errs.CodeInternalError, "error saving document record", err)
	}
	return doc, nil
}

func (s *documentService) storedSize(relPath string) (int64, error) {
	f, err := s.store.Open(relPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return f.Seek(0, io.SeekEnd)
}

func (s *documentService) cleanupStored(ctx context.Context, relPath string) {
	if err := s.store.Delete(relPath); err != nil {
		logger.Logger(ctx).Warn("error cleaning up stored file", "path", relPath, "err", err)
	}
}

func (s *documentService) ListByTransaction(
	ctx context.Context,
	transactionId uuid.UUID,
) ([]model.TransactionDocument, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetByTransactionId(ctx, budgetId, transactionId)
}

func (s *documentService) Content(
	ctx context.Context,
	id uuid.UUID,
) (*model.TransactionDocument, io.ReadSeekCloser, error) {
	budgetId := utils.MustBudgetID(ctx)
	doc, err := s.repo.GetById(ctx, budgetId, id)
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, "document not found", err)
	}
	f, err := s.store.Open(doc.StoragePath)
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, "error opening document", err)
	}
	return doc, f, nil
}

func (s *documentService) Delete(ctx context.Context, id uuid.UUID) error {
	budgetId := utils.MustBudgetID(ctx)
	doc, err := s.repo.GetById(ctx, budgetId, id)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "document not found", err)
	}
	if err := s.repo.DeleteById(ctx, budgetId, id); err != nil {
		return errs.Wrap(errs.CodeInternalError, "error deleting document", err)
	}
	// row is soft-deleted; removing the file is best-effort
	s.cleanupStored(ctx, doc.StoragePath)
	return nil
}
