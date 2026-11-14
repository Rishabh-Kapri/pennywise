package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type StatementImportRepository interface {
	Create(ctx context.Context, imp model.StatementImport) (*model.StatementImport, error)
	GetByID(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.StatementImport, error)
	// FindByHash returns a live (not failed, not deleted) import of the same
	// file, or nil.
	FindByHash(ctx context.Context, budgetId uuid.UUID, hash string) (*model.StatementImport, error)
	// List omits sections and source text; RowCount is filled from the JSON.
	List(ctx context.Context, budgetId uuid.UUID, limit int) ([]model.StatementImport, error)
	UpdateProgress(ctx context.Context, id uuid.UUID, pagesProcessed int) error
	// Complete and Fail change only an import that is still processing.
	Complete(ctx context.Context, id uuid.UUID, sections []model.StatementSection, droppedRows int) error
	Fail(ctx context.Context, id uuid.UUID, message string) error
	Restart(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
	UpdateSections(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, sections []model.StatementSection) error
	Delete(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
	// FailInterrupted marks every processing import without a job failed.
	// Those were read in the API's process, so at startup they were cut off by
	// the restart. An import with a job carries on in its job.
	FailInterrupted(ctx context.Context, message string) (int64, error)
}
