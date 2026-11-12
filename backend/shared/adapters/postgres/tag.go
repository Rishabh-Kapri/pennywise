package postgres

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tagRepo struct {
	BaseRepository
}

func NewTagRepository(pool *pgxpool.Pool) repository.TagRepository {
	return &tagRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *tagRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Tag, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
		SELECT id, name, budget_id, color, created_at, updated_at
		FROM tags WHERE budget_id = $1 AND deleted = FALSE
		ORDER BY name ASC`,
		budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var tag model.Tag
		err := rows.Scan(&tag.ID, &tag.Name, &tag.BudgetID, &tag.Color, &tag.CreatedAt, &tag.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *tagRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Tag, error) {
	rows, err := r.Executor(ctx).Query(
		ctx,
		`SELECT id, name, budget_id, color, created_at, updated_at FROM tags
		 WHERE budget_id = $1 AND deleted = FALSE AND LOWER(name) LIKE LOWER('%' || $2 || '%')
		 ORDER BY name ASC`,
		budgetId, query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var tag model.Tag
		err := rows.Scan(&tag.ID, &tag.Name, &tag.BudgetID, &tag.Color, &tag.CreatedAt, &tag.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *tagRepo) GetById(ctx context.Context, budgetId, id uuid.UUID) (*model.Tag, error) {
	var tag model.Tag
	err := r.Executor(ctx).QueryRow(
		ctx, `
		SELECT id, name, budget_id, color, created_at, updated_at
		FROM tags
		WHERE id = $1 AND budget_id = $2 AND deleted = FALSE
		`, id, budgetId,
	).Scan(&tag.ID, &tag.Name, &tag.BudgetID, &tag.Color, &tag.CreatedAt, &tag.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *tagRepo) Create(ctx context.Context, tag model.Tag) (*model.Tag, error) {
	var createdTag model.Tag
	err := r.Executor(ctx).QueryRow(
		ctx, `
		INSERT INTO tags (name, budget_id, color, deleted, created_at, updated_at)
		VALUES ($1, $2, $3, FALSE, NOW(), NOW())
		RETURNING id, name, budget_id, color, created_at, updated_at
		`, tag.Name, tag.BudgetID, tag.Color,
	).Scan(&createdTag.ID, &createdTag.Name, &createdTag.BudgetID, &createdTag.Color, &createdTag.CreatedAt, &createdTag.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &createdTag, nil
}

func (r *tagRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, tag model.Tag) error {
	cmdTag, err := r.Executor(ctx).Exec(
		ctx,
		`UPDATE tags SET
		   name = $1,
		   color = $2,
		   updated_at = NOW()
		WHERE id = $3 AND budget_id = $4`,
		tag.Name, tag.Color, id, budgetId,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return notFoundError("Tag")
	}
	return nil
}

func (r *tagRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	cmdTag, err := r.Executor(ctx).Exec(
		ctx,
		`UPDATE tags SET
		   deleted = TRUE,
		   updated_at = NOW()
		WHERE id = $1 AND budget_id = $2`,
		id, budgetId,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return notFoundError("Tag")
	}
	return nil
}
