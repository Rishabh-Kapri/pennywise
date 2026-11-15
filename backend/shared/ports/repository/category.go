package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type CategoryRepository interface {
	GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Category, error)
	GetAllSimplified(ctx context.Context, budgetId uuid.UUID) ([]model.CategorySimplified, error)
	FindClosestSimplified(ctx context.Context, budgetId uuid.UUID, name string, limit int) ([]model.CategoryNameMatch, error)
	GetInflowBalance(ctx context.Context, budgetId uuid.UUID) (float64, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Category, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error)
	GetByIdSimplified(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error)
	Create(ctx context.Context, category model.Category) (*model.Category, error)
	DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error
	Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, category model.Category) error
	// UpdateDetails sets the note and/or description; a nil field is left as is.
	UpdateDetails(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, note *string, description *string) error
	// GetDescriptionContext gathers what an AI description suggestion needs:
	// the category and group names, the group's other categories, and the
	// payees the category's transactions most often go to.
	GetDescriptionContext(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, payeeLimit int) (*model.CategoryDescriptionContext, error)
}
