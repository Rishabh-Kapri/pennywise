package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const getBudgetToolName = "get_budget_info"

type CategorySpend struct {
	Name       string  `json:"name"`
	TotalSpend float64 `json:"totalSpend"`
}

type BudgetInfo struct {
	Categories []string `json:"categories"`
	PayeeNames []string        `json:"payeeNames"`
}

type BudgetToolArgs struct {
	DateRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"dateRange"`
}

type GetBudgetInfoTool struct {
	db *pgxpool.Pool
}

func NewGetBudgetInfoTool(db *pgxpool.Pool) Tool {
	return GetBudgetInfoTool{db: db}
}

func (t GetBudgetInfoTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name:        getBudgetToolName,
		Description: "Return the user's budget info with categories & payees used for a specific date range. Only call this tool when date range is known.",
		InputSchema: sharedModel.ToolSchema{
			Type: "object",
			Properties: map[string]sharedModel.ToolSchema{
				"dateRange": {
					Type: "object",
					Properties: map[string]sharedModel.ToolSchema{
						"start": {
							Type:        "string",
							Description: "Start date to query from. Empty string when not applicable.",
						},
						"end": {
							Type:        "string",
							Description: "End date to query to. Empty string when not applicable.",
						},
					},
					Required: []string{"start", "end"},
				},
			},
			Required: []string{"dateRange"},
		},
	}
}

func (t GetBudgetInfoTool) fetchCategories(
	ctx context.Context,
	budgetID uuid.UUID,
	args BudgetToolArgs,
) (categories []string, err error) {
	categoryRows, err := t.db.Query(ctx, `
			SELECT
			c.name,
			COALESCE(SUM(t.amount), 0) AS total_spend
			FROM transactions t
			JOIN categories c ON t.category_id = c.id AND c.is_system = false AND c.deleted = false
			WHERE t.budget_id = $1
			AND t.date >= $2
			AND t.date <= $3
			AND t.deleted = false
			GROUP BY c.name
			ORDER BY total_spend ASC
			`, budgetID, args.DateRange.Start, args.DateRange.End)
	if err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to execute tool get_budget_info", err)
	}
	defer categoryRows.Close()
	for categoryRows.Next() {
		var name string
		if err := categoryRows.Scan(&name); err != nil {
			return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to scan category row", err)
		}
		categories = append(categories, name)
	}
	if err := categoryRows.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to scan category rows", err)
	}

	return categories, nil
}

func (t GetBudgetInfoTool) fetchPayees(
	ctx context.Context,
	budgetID uuid.UUID,
	args BudgetToolArgs,
) (payeeNames []string, err error) {
	payeeRows, err := t.db.Query(ctx, `
		SELECT DISTINCT p.name
		FROM transactions t
		JOIN payees p ON t.payee_id = p.id
		WHERE t.budget_id = $1
		  AND t.date >= $2
		  AND t.date <= $3
		  AND t.deleted = false
		  AND p.name IS NOT NULL
		ORDER BY p.name
	`, budgetID, args.DateRange.Start, args.DateRange.End)
	if err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to execute tool get_budget_info", err)
	}
	defer payeeRows.Close()

	for payeeRows.Next() {
		var name string
		if err := payeeRows.Scan(&name); err != nil {
			return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to scan payee row", err)
		}
		payeeNames = append(payeeNames, name)
	}
	if err := payeeRows.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to scan payee rows", err)
	}

	return payeeNames, nil
}

func (t GetBudgetInfoTool) Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	var args BudgetToolArgs
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse get_budget_info arguments", err)
	}
	if args.DateRange.Start == "" || args.DateRange.End == "" {
		return nil, errs.New(errs.CodeToolExecuteFail, "date range is required")
	}

	budgetID := utils.MustBudgetID(ctx)

	var categories []string
	var payeeNames []string
	var catErr, payeeErr error

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		categories, catErr = t.fetchCategories(ctx, budgetID, args)
	}()

	go func() {
		defer wg.Done()
		payeeNames, payeeErr = t.fetchPayees(ctx, budgetID, args)
	}()

	wg.Wait()

	if catErr != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to fetch categories", catErr)
	}
	if payeeErr != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to fetch payees", payeeErr)
	}

	return jsonToolResult(call, getBudgetToolName, BudgetInfo{
		Categories: categories,
		PayeeNames: payeeNames,
	})
}

func (t GetBudgetInfoTool) GetNormalizedName(isDone bool) string {
	if isDone {
		return "Loaded budget context"
	}
	return "Loading budget context..."
}

func (t GetBudgetInfoTool) Normalize(
	call sharedModel.ToolCall,
	result json.RawMessage,
) (*sharedModel.ToolResultNormalized, error) {
	var args struct {
		DateRange struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"dateRange"`
	}
	if len(call.Arguments) > 0 {
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, "parse get_budget_info normalized arguments", err)
		}
	}

	var toolResult sharedModel.ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse get_budget_info normalized result", err)
	}

	var budgetInfo BudgetInfo
	for _, block := range toolResult.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		if err := json.Unmarshal([]byte(block.Text), &budgetInfo); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, "parse get_budget_info result", err)
		}
		break
	}

	categoryCount := len(budgetInfo.Categories)
	payeeCount := len(budgetInfo.PayeeNames)
	summary := "Loaded budget context"
	if categoryCount > 0 || payeeCount > 0 {
		summary = "Found "
		switch {
		case categoryCount > 0 && payeeCount > 0:
			summary += pluralizeCount(categoryCount, "category", "categories") + " and " +
				pluralizeCount(payeeCount, "payee", "payees")
		case categoryCount > 0:
			summary += pluralizeCount(categoryCount, "category", "categories")
		default:
			summary += pluralizeCount(payeeCount, "payee", "payees")
		}
	}

	normalized := map[string]any{
		"categoryCount": categoryCount,
		"payeeCount":    payeeCount,
	}
	if args.DateRange.Start != "" || args.DateRange.End != "" {
		normalized["dateRange"] = map[string]string{
			"start": args.DateRange.Start,
			"end":   args.DateRange.End,
		}
	}

	normalizedJSON, err := json.Marshal(normalized)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "marshal get_budget_info normalized result", err)
	}

	return &sharedModel.ToolResultNormalized{
		DisplayName: t.GetNormalizedName(true),
		Summary:     summary,
		Result:      json.RawMessage(normalizedJSON),
	}, nil
}

func pluralizeCount(count int, singular string, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(count) + " " + plural
}
