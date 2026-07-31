package db

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionDocumentRepository interface {
	BaseRepositoryInterface
	GetByTransactionId(ctx context.Context, budgetId uuid.UUID, transactionId uuid.UUID) ([]model.TransactionDocument, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.TransactionDocument, error)
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
