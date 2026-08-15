package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
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
	// List returns a filtered page of every document in the budget.
	List(ctx context.Context, filter model.DocumentFilter) (model.DocumentLibraryResponse, error)
	// Content returns the document metadata and a reader over its bytes.
	Content(ctx context.Context, id uuid.UUID) (*model.TransactionDocument, io.ReadSeekCloser, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type documentService struct {
	repo      repository.TransactionDocumentRepository
	txnRepo   repository.TransactionRepository
	payeeRepo repository.PayeesRepository
	store     storage.Store
}

func NewDocumentService(
	repo repository.TransactionDocumentRepository,
	txnRepo repository.TransactionRepository,
	payeeRepo repository.PayeesRepository,
	store storage.Store,
) DocumentService {
	return &documentService{repo: repo, txnRepo: txnRepo, payeeRepo: payeeRepo, store: store}
}

// nonAlphanumeric strips everything that would make a filename awkward to read
// or to handle in a shell or bucket browser. Case is preserved.
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// documentBaseName builds "<Payee>_<YYYYMMDD>-<n>" for a transaction's nth
// document, e.g. "McDonalds_20260819-1".
//
// The index is always present, even for the first document. Numbering only on
// the second upload would mean renaming the first one retroactively, which
// would invalidate a storage key already written to the row.
func (s *documentService) documentBaseName(
	ctx context.Context,
	budgetId uuid.UUID,
	txn *model.Transaction,
	index int,
) string {
	payeeName := ""
	if txn != nil && txn.PayeeID != nil {
		if found, err := s.payeeRepo.GetById(ctx, budgetId, *txn.PayeeID); err == nil && found != nil {
			payeeName = found.Name
		}
	}

	txnDate := ""
	if txn != nil {
		txnDate = string(txn.Date)
	}
	return formatDocumentName(payeeName, txnDate, index)
}

// formatDocumentName produces "<Payee>_<YYYYMMDD>-<n>", e.g. "McDonalds_20260819-1".
//
// Falls back to "Receipt" for an unknown payee and to today for an unparseable
// date, so a name is always produced -- a missing payee should not block an
// upload.
func formatDocumentName(payeeName string, txnDate string, index int) string {
	payee := nonAlphanumeric.ReplaceAllString(payeeName, "")
	if payee == "" {
		payee = "Receipt"
	}

	// Transaction date, not upload date: a receipt belongs to when the money
	// moved, which is what someone browsing the bucket is looking for.
	date := time.Now().Format("20060102")
	if parsed, err := time.Parse("2006-01-02", txnDate); err == nil {
		date = parsed.Format("20060102")
	} else if parsed, err := time.Parse(time.RFC3339, txnDate); err == nil {
		date = parsed.Format("20060102")
	}

	if index < 1 {
		index = 1
	}
	return fmt.Sprintf("%s_%s-%d", payee, date, index)
}

// nextDocumentIndex is the 1-based position of the document about to be added.
// Counting rather than tracking a sequence can collide if two uploads race;
// the storage key is scoped per transaction so a collision would overwrite
// within one transaction only, which is an acceptable trade for readable names.
func (s *documentService) nextDocumentIndex(
	ctx context.Context,
	budgetId uuid.UUID,
	transactionId uuid.UUID,
) int {
	existing, err := s.repo.GetByTransactionId(ctx, budgetId, transactionId)
	if err != nil {
		return 1
	}
	return len(existing) + 1
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
	txn, err := s.txnRepo.GetById(ctx, budgetId, transactionId)
	if err != nil {
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
	baseName := s.documentBaseName(ctx, budgetId, txn, s.nextDocumentIndex(ctx, budgetId, transactionId))
	// Key is scoped per transaction so two transactions with the same payee on
	// the same day cannot collide on "<Payee>_<date>-1".
	relPath, err := s.store.Save(
		path.Join(budgetId.String(), transactionId.String()),
		baseName+ext,
		io.MultiReader(strings.NewReader(string(head)), body),
	)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error storing file", err)
	}

	// The stored name is also what the browser sees on download, so the
	// client-supplied name is deliberately discarded.
	fileName = baseName + ext

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
	txn, err := s.txnRepo.GetById(ctx, budgetId, transactionId)
	if err != nil {
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
	baseName := s.documentBaseName(ctx, budgetId, txn, s.nextDocumentIndex(ctx, budgetId, transactionId))
	relPath, err := s.store.Save(
		path.Join(budgetId.String(), transactionId.String()),
		baseName+".pdf",
		bytes.NewReader(pdfBytes),
	)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error storing scan", err)
	}

	doc, err := s.repo.Create(ctx, model.TransactionDocument{
		ID:            docId,
		BudgetID:      budgetId,
		TransactionID: transactionId,
		FileName:      baseName + ".pdf",
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

// MaxDocumentPageSize bounds a library page. A tile is a thumbnail fetch of
// its own, so an unbounded page would fan out into hundreds of content
// requests behind one list call.
const MaxDocumentPageSize = 100

func (s *documentService) List(
	ctx context.Context,
	filter model.DocumentFilter,
) (model.DocumentLibraryResponse, error) {
	budgetId := utils.MustBudgetID(ctx)

	if !filter.Kind.Valid() {
		return model.DocumentLibraryResponse{}, errs.New(
			errs.CodeInvalidArgument,
			"unknown document type %q; allowed: image, pdf",
			string(filter.Kind),
		)
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > MaxDocumentPageSize {
		filter.Limit = MaxDocumentPageSize
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	return s.repo.Search(ctx, budgetId, filter)
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
