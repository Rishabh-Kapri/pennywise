package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionDocumentRepository interface {
	BaseRepositoryInterface
	GetByTransactionId(ctx context.Context, budgetId uuid.UUID, transactionId uuid.UUID) ([]model.TransactionDocument, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.TransactionDocument, error)
	// Search lists every document in the budget with its transaction context.
	Search(ctx context.Context, budgetId uuid.UUID, filter model.DocumentFilter) (model.DocumentLibraryResponse, error)
	Create(ctx context.Context, doc model.TransactionDocument) (*model.TransactionDocument, error)
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
}

type transactionDocumentRepo struct {
	BaseRepository
}

func NewTransactionDocumentRepository(pool *pgxpool.Pool) TransactionDocumentRepository {
	return &transactionDocumentRepo{BaseRepository: NewBaseRepository(pool)}
}

const transactionDocumentColumns = `id, budget_id, transaction_id, file_name, mime_type, size_bytes, storage_path, deleted, created_at, updated_at`

func scanTransactionDocument(row interface{ Scan(dest ...any) error }) (model.TransactionDocument, error) {
	var doc model.TransactionDocument
	err := row.Scan(
		&doc.ID,
		&doc.BudgetID,
		&doc.TransactionID,
		&doc.FileName,
		&doc.MimeType,
		&doc.SizeBytes,
		&doc.StoragePath,
		&doc.Deleted,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	return doc, err
}

func (r *transactionDocumentRepo) GetByTransactionId(
	ctx context.Context,
	budgetId uuid.UUID,
	transactionId uuid.UUID,
) ([]model.TransactionDocument, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT `+transactionDocumentColumns+`
		 FROM transaction_documents
		 WHERE budget_id = $1 AND transaction_id = $2 AND deleted = FALSE
		 ORDER BY created_at ASC`,
		budgetId, transactionId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []model.TransactionDocument{}
	for rows.Next() {
		doc, err := scanTransactionDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (r *transactionDocumentRepo) GetById(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
) (*model.TransactionDocument, error) {
	doc, err := scanTransactionDocument(r.Executor(nil).QueryRow(
		ctx,
		`SELECT `+transactionDocumentColumns+`
		 FROM transaction_documents
		 WHERE budget_id = $1 AND id = $2 AND deleted = FALSE`,
		budgetId, id,
	))
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// documentLibraryColumns mirrors transactionDocumentColumns but qualified, so
// the joined query can select the same fields without ambiguity.
const documentLibraryColumns = `transaction_documents.id,
	transaction_documents.budget_id,
	transaction_documents.transaction_id,
	transaction_documents.file_name,
	transaction_documents.mime_type,
	transaction_documents.size_bytes,
	transaction_documents.storage_path,
	transaction_documents.deleted,
	transaction_documents.created_at,
	transaction_documents.updated_at`

// documentSearchConditions builds the WHERE fragment shared by the page query
// and its COUNT, so the two can never drift and report a total the page
// contradicts. Both start at $1, so callers pass the same args to each.
//
// The transactions join is INNER, not LEFT: a document whose transaction was
// hard-deleted has no context to show and no target to click through to, so it
// has no place in the library.
func documentSearchConditions(budgetId uuid.UUID, filter model.DocumentFilter) (string, []any) {
	where := `transaction_documents.budget_id = $1
		AND transaction_documents.deleted = FALSE
		AND transactions.deleted = FALSE`
	args := []any{budgetId}

	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		// COALESCE so an unset payee matches on file name alone rather than
		// dropping the row: NULL ILIKE anything is NULL, not false.
		where += fmt.Sprintf(
			` AND (transaction_documents.file_name ILIKE $%d OR COALESCE(payees.name, '') ILIKE $%d)`,
			len(args), len(args),
		)
	}

	switch filter.Kind {
	case model.DocumentKindImage:
		where += ` AND transaction_documents.mime_type LIKE 'image/%'`
	case model.DocumentKindPDF:
		where += ` AND transaction_documents.mime_type = 'application/pdf'`
	}

	if filter.StartDate != "" {
		args = append(args, filter.StartDate)
		where += fmt.Sprintf(` AND transactions.date >= $%d`, len(args))
	}
	if filter.EndDate != "" {
		args = append(args, filter.EndDate)
		where += fmt.Sprintf(` AND transactions.date <= $%d`, len(args))
	}

	return where, args
}

const documentSearchJoins = `FROM transaction_documents
	JOIN transactions ON transactions.id = transaction_documents.transaction_id
	LEFT JOIN payees ON transactions.payee_id = payees.id
	LEFT JOIN accounts ON transactions.account_id = accounts.id
	LEFT JOIN categories ON transactions.category_id = categories.id`

func (r *transactionDocumentRepo) Search(
	ctx context.Context,
	budgetId uuid.UUID,
	filter model.DocumentFilter,
) (model.DocumentLibraryResponse, error) {
	where, args := documentSearchConditions(budgetId, filter)

	var total int
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT COUNT(*) `+documentSearchJoins+` WHERE `+where,
		args...,
	).Scan(&total)
	if err != nil {
		return model.DocumentLibraryResponse{}, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Fetch one past the page so HasMore is known without a second count
	// against a total that a concurrent upload may already have moved.
	pageArgs := append(append([]any{}, args...), limit+1, offset)
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT `+documentLibraryColumns+`,
			transactions.date,
			transactions.amount,
			payees.name,
			accounts.name,
			categories.name
		 `+documentSearchJoins+`
		 WHERE `+where+`
		 ORDER BY transactions.date DESC, transaction_documents.created_at DESC, transaction_documents.id DESC
		 LIMIT $`+strconv.Itoa(len(pageArgs)-1)+` OFFSET $`+strconv.Itoa(len(pageArgs)),
		pageArgs...,
	)
	if err != nil {
		return model.DocumentLibraryResponse{}, err
	}
	defer rows.Close()

	items := []model.DocumentListItem{}
	for rows.Next() {
		var item model.DocumentListItem
		err := rows.Scan(
			&item.ID,
			&item.BudgetID,
			&item.TransactionID,
			&item.FileName,
			&item.MimeType,
			&item.SizeBytes,
			&item.StoragePath,
			&item.Deleted,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.TransactionDate,
			&item.TransactionAmount,
			&item.PayeeName,
			&item.AccountName,
			&item.CategoryName,
		)
		if err != nil {
			return model.DocumentLibraryResponse{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return model.DocumentLibraryResponse{}, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	return model.DocumentLibraryResponse{
		Data:    items,
		Total:   total,
		HasMore: hasMore,
	}, nil
}

func (r *transactionDocumentRepo) Create(
	ctx context.Context,
	doc model.TransactionDocument,
) (*model.TransactionDocument, error) {
	created, err := scanTransactionDocument(r.Executor(nil).QueryRow(
		ctx,
		`INSERT INTO transaction_documents (
			budget_id, transaction_id, file_name, mime_type, size_bytes, storage_path
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+transactionDocumentColumns,
		doc.BudgetID,
		doc.TransactionID,
		doc.FileName,
		doc.MimeType,
		doc.SizeBytes,
		doc.StoragePath,
	))
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *transactionDocumentRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	cmdTag, err := r.Executor(nil).Exec(
		ctx,
		`UPDATE transaction_documents SET
			deleted = TRUE,
			updated_at = NOW()
		 WHERE budget_id = $1 AND id = $2 AND deleted = FALSE`,
		budgetId, id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("Document not found")
	}
	return nil
}
