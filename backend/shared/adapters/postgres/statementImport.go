package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type statementImportRepo struct {
	BaseRepository
}

func NewStatementImportRepository(pool *pgxpool.Pool) repository.StatementImportRepository {
	return &statementImportRepo{BaseRepository: NewBaseRepository(pool)}
}

const statementImportColumns = `
	id, budget_id, file_name, file_hash, status, error, page_count, pages_processed,
	default_account_id, sections, dropped_rows, source_text, created_at, updated_at`

func scanStatementImport(row pgx.Row) (*model.StatementImport, error) {
	var imp model.StatementImport
	var sections []byte
	err := row.Scan(
		&imp.ID,
		&imp.BudgetID,
		&imp.FileName,
		&imp.FileHash,
		&imp.Status,
		&imp.Error,
		&imp.PageCount,
		&imp.PagesProcessed,
		&imp.DefaultAccountID,
		&sections,
		&imp.DroppedRows,
		&imp.SourceText,
		&imp.CreatedAt,
		&imp.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(sections, &imp.Sections); err != nil {
		return nil, err
	}
	if imp.Sections == nil {
		imp.Sections = []model.StatementSection{}
	}
	for _, s := range imp.Sections {
		imp.RowCount += len(s.Rows)
	}
	return &imp, nil
}

func (r *statementImportRepo) Create(ctx context.Context, imp model.StatementImport) (*model.StatementImport, error) {
	sections, err := json.Marshal(nonNilSections(imp.Sections))
	if err != nil {
		return nil, err
	}
	return scanStatementImport(r.Executor(ctx).QueryRow(ctx, `
		INSERT INTO statement_imports (
			budget_id, file_name, file_hash, status, page_count, default_account_id, sections, source_text
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+statementImportColumns,
		imp.BudgetID,
		imp.FileName,
		imp.FileHash,
		imp.Status,
		imp.PageCount,
		imp.DefaultAccountID,
		sections,
		imp.SourceText,
	))
}

func (r *statementImportRepo) GetByID(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.StatementImport, error) {
	return scanStatementImport(r.Executor(ctx).QueryRow(ctx, `
		SELECT `+statementImportColumns+`
		FROM statement_imports
		WHERE budget_id = $1 AND id = $2 AND deleted = FALSE`,
		budgetId, id,
	))
}

func (r *statementImportRepo) FindByHash(ctx context.Context, budgetId uuid.UUID, hash string) (*model.StatementImport, error) {
	imp, err := scanStatementImport(r.Executor(ctx).QueryRow(ctx, `
		SELECT `+statementImportColumns+`
		FROM statement_imports
		WHERE budget_id = $1 AND file_hash = $2 AND deleted = FALSE AND status <> 'failed'
		ORDER BY created_at DESC
		LIMIT 1`,
		budgetId, hash,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return imp, err
}

func (r *statementImportRepo) List(ctx context.Context, budgetId uuid.UUID, limit int) ([]model.StatementImport, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT
			id, budget_id, file_name, status, error, page_count, pages_processed,
			default_account_id, dropped_rows, created_at, updated_at,
			COALESCE((
				SELECT SUM(jsonb_array_length(COALESCE(s->'rows', '[]'::jsonb)))
				FROM jsonb_array_elements(sections) s
			), 0)::int AS row_count
		FROM statement_imports
		WHERE budget_id = $1 AND deleted = FALSE
		ORDER BY created_at DESC
		LIMIT $2`,
		budgetId, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	imports := make([]model.StatementImport, 0)
	for rows.Next() {
		var imp model.StatementImport
		if err := rows.Scan(
			&imp.ID,
			&imp.BudgetID,
			&imp.FileName,
			&imp.Status,
			&imp.Error,
			&imp.PageCount,
			&imp.PagesProcessed,
			&imp.DefaultAccountID,
			&imp.DroppedRows,
			&imp.CreatedAt,
			&imp.UpdatedAt,
			&imp.RowCount,
		); err != nil {
			return nil, err
		}
		imp.Sections = []model.StatementSection{}
		imports = append(imports, imp)
	}
	return imports, rows.Err()
}

func (r *statementImportRepo) UpdateProgress(ctx context.Context, id uuid.UUID, pagesProcessed int) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		UPDATE statement_imports SET pages_processed = $2, updated_at = NOW()
		WHERE id = $1`,
		id, pagesProcessed,
	)
	return err
}

func (r *statementImportRepo) Complete(
	ctx context.Context,
	id uuid.UUID,
	sections []model.StatementSection,
	droppedRows int,
) error {
	body, err := json.Marshal(nonNilSections(sections))
	if err != nil {
		return err
	}
	_, err = r.Executor(ctx).Exec(ctx, `
		UPDATE statement_imports
		SET status = 'ready', error = NULL, sections = $2, dropped_rows = $3,
			pages_processed = page_count, updated_at = NOW()
		WHERE id = $1`,
		id, body, droppedRows,
	)
	return err
}

func (r *statementImportRepo) Fail(ctx context.Context, id uuid.UUID, message string) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		UPDATE statement_imports SET status = 'failed', error = $2, updated_at = NOW()
		WHERE id = $1`,
		id, message,
	)
	return err
}

func (r *statementImportRepo) Restart(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	tag, err := r.Executor(ctx).Exec(ctx, `
		UPDATE statement_imports
		SET status = 'processing', error = NULL, pages_processed = 0, updated_at = NOW()
		WHERE budget_id = $1 AND id = $2 AND deleted = FALSE AND status = 'failed'`,
		budgetId, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNoRows
	}
	return nil
}

func (r *statementImportRepo) UpdateSections(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
	sections []model.StatementSection,
) error {
	body, err := json.Marshal(nonNilSections(sections))
	if err != nil {
		return err
	}
	tag, err := r.Executor(ctx).Exec(ctx, `
		UPDATE statement_imports SET sections = $3, updated_at = NOW()
		WHERE budget_id = $1 AND id = $2 AND deleted = FALSE`,
		budgetId, id, body,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNoRows
	}
	return nil
}

func (r *statementImportRepo) Delete(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	// Clearing the text with the soft delete: it is a full account history.
	tag, err := r.Executor(ctx).Exec(ctx, `
		UPDATE statement_imports
		SET deleted = TRUE, source_text = '', sections = '[]'::jsonb, updated_at = NOW()
		WHERE budget_id = $1 AND id = $2 AND deleted = FALSE`,
		budgetId, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNoRows
	}
	return nil
}

func (r *statementImportRepo) FailInterrupted(ctx context.Context, message string) (int64, error) {
	tag, err := r.Executor(ctx).Exec(ctx, `
		UPDATE statement_imports SET status = 'failed', error = $1, updated_at = NOW()
		WHERE status = 'processing' AND deleted = FALSE`,
		message,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func nonNilSections(sections []model.StatementSection) []model.StatementSection {
	if sections == nil {
		return []model.StatementSection{}
	}
	return sections
}
