/*
* Builds the context required by the agent
 */
package context

import (
	"context"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetContext struct {
	Categories []sharedModel.CategorySimplified
	Accounts   []sharedModel.AccountSimplified
}

type EntityResolutionContext struct {
	Categories []sharedModel.CategorySimplified `json:"categories"`
	Accounts   []sharedModel.AccountSimplified  `json:"accounts"`
	Payees     []PayeeCandidate                 `json:"payees"`
}

type PayeeCandidate struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// CategorySpend is a category name and its total spend for a date window.
type CategorySpend struct {
	Name       string  `json:"name"`
	TotalSpend float64 `json:"totalSpend"`
}

// ScopedContext is the minimal budget context for a resolved date window.
// Only category/payee display names and spend totals are included — no raw
// transaction rows, account numbers, or balances.
type ScopedContext struct {
	Categories []CategorySpend `json:"categories"`
	PayeeNames []string        `json:"payeeNames"`
}

type ContextBuilder interface {
	GetSystemPrompt() string
	GetBudgetContext(ctx context.Context, budgetId uuid.UUID) (BudgetContext, error)
	GetEntityResolutionContext(ctx context.Context, budgetId uuid.UUID) (EntityResolutionContext, error)
	// GetCategoryGroupNames returns a deduplicated list of non-hidden, non-system
	// category group names for the given budget. Safe to send to a cloud LLM —
	// no IDs, balances, or individual category names.
	GetCategoryGroupNames(ctx context.Context, budgetId uuid.UUID) ([]string, error)
	// GetScopedContext loads only the categories and payees that have
	// transactions in the resolved date window, plus summed spend per category.
	// No raw rows or individual amounts are returned.
	GetScopedContext(ctx context.Context, budgetId uuid.UUID, dateRange *sharedModel.DateRange) (*ScopedContext, error)
}

type contextBuilder struct {
	pool              *pgxpool.Pool
	accountRepo       db.AccountRepository
	budgetRepo        db.BudgetRepository
	categoryRepo      db.CategoryRepository
	payeeRepo         db.PayeesRepository
	categoryGroupRepo db.CategoryGroupRepository
}

func NewContextBuilder(
	pool *pgxpool.Pool,
	accountRepo db.AccountRepository,
	budgetRepo db.BudgetRepository,
	categoryRepo db.CategoryRepository,
	payeeRepo db.PayeesRepository,
	categoryGroupRepo db.CategoryGroupRepository,
) ContextBuilder {
	return &contextBuilder{
		pool:              pool,
		accountRepo:       accountRepo,
		budgetRepo:        budgetRepo,
		categoryRepo:      categoryRepo,
		payeeRepo:         payeeRepo,
		categoryGroupRepo: categoryGroupRepo,
	}
}

func (b *contextBuilder) GetSystemPrompt() string {
	return SystemPrompt
}

// GetScopedContext queries only the categories and payees that have
// transactions in [dateRange.From, dateRange.To], along with summed spend per
// category. Category/payee display names and aggregates only — no raw rows.
func (b *contextBuilder) GetScopedContext(
	ctx context.Context,
	budgetId uuid.UUID,
	dateRange *sharedModel.DateRange,
) (*ScopedContext, error) {
	if dateRange == nil {
		return nil, fmt.Errorf("GetScopedContext: dateRange must not be nil")
	}

	// Category spend totals for the window.
	categoryRows, err := b.pool.Query(ctx, `
		SELECT
			c.name,
			COALESCE(SUM(t.amount), 0) AS total_spend
		FROM transactions t
		JOIN categories c ON t.category_id = c.id
		WHERE t.budget_id = $1
		  AND t.date >= $2
		  AND t.date <= $3
		  AND t.deleted = false
		GROUP BY c.name
		ORDER BY total_spend ASC
	`, budgetId, dateRange.From, dateRange.To)
	if err != nil {
		return nil, fmt.Errorf("GetScopedContext: category query: %w", err)
	}
	defer categoryRows.Close()

	var categories []CategorySpend
	for categoryRows.Next() {
		var cs CategorySpend
		if err := categoryRows.Scan(&cs.Name, &cs.TotalSpend); err != nil {
			return nil, fmt.Errorf("GetScopedContext: category scan: %w", err)
		}
		categories = append(categories, cs)
	}
	if err := categoryRows.Err(); err != nil {
		return nil, fmt.Errorf("GetScopedContext: category rows: %w", err)
	}

	// Distinct payee display names active in the window.
	payeeRows, err := b.pool.Query(ctx, `
		SELECT DISTINCT p.name
		FROM transactions t
		JOIN payees p ON t.payee_id = p.id
		WHERE t.budget_id = $1
		  AND t.date >= $2
		  AND t.date <= $3
		  AND t.deleted = false
		  AND p.name IS NOT NULL
		ORDER BY p.name
	`, budgetId, dateRange.From, dateRange.To)
	if err != nil {
		return nil, fmt.Errorf("GetScopedContext: payee query: %w", err)
	}
	defer payeeRows.Close()

	var payeeNames []string
	for payeeRows.Next() {
		var name string
		if err := payeeRows.Scan(&name); err != nil {
			return nil, fmt.Errorf("GetScopedContext: payee scan: %w", err)
		}
		payeeNames = append(payeeNames, name)
	}
	if err := payeeRows.Err(); err != nil {
		return nil, fmt.Errorf("GetScopedContext: payee rows: %w", err)
	}

	return &ScopedContext{
		Categories: categories,
		PayeeNames: payeeNames,
	}, nil
}

func (b *contextBuilder) GetBudgetContext(ctx context.Context, budgetId uuid.UUID) (BudgetContext, error) {
	var budgetContext BudgetContext
	categories, err := b.categoryRepo.GetAllSimplified(ctx, budgetId)
	if err != nil {
		return budgetContext, err
	}
	accounts, err := b.accountRepo.GetAllSimplified(ctx, budgetId)
	if err != nil {
		return budgetContext, err
	}

	budgetContext.Categories = append(budgetContext.Categories, categories...)
	budgetContext.Accounts = append(budgetContext.Accounts, accounts...)

	return budgetContext, nil
}

func (b *contextBuilder) GetEntityResolutionContext(ctx context.Context, budgetId uuid.UUID) (EntityResolutionContext, error) {
	var resolverContext EntityResolutionContext

	categories, err := b.categoryRepo.GetAllSimplified(ctx, budgetId)
	if err != nil {
		return resolverContext, err
	}
	accounts, err := b.accountRepo.GetAllSimplified(ctx, budgetId)
	if err != nil {
		return resolverContext, err
	}

	resolverContext.Categories = append(resolverContext.Categories, categories...)
	resolverContext.Accounts = append(resolverContext.Accounts, accounts...)

	if b.payeeRepo == nil {
		return resolverContext, nil
	}

	payees, err := b.payeeRepo.GetAll(ctx, budgetId)
	if err != nil {
		return resolverContext, err
	}
	resolverContext.Payees = make([]PayeeCandidate, 0, len(payees))
	for _, payee := range payees {
		resolverContext.Payees = append(resolverContext.Payees, PayeeCandidate{
			ID:   payee.ID,
			Name: payee.Name,
		})
	}

	return resolverContext, nil
}

func (b *contextBuilder) GetCategoryGroupNames(ctx context.Context, budgetId uuid.UUID) ([]string, error) {
	groups, err := b.categoryGroupRepo.GetAll(ctx, budgetId)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		if g.IsSystem || g.Hidden {
			continue
		}
		names = append(names, g.Name)
	}
	return names, nil
}
