package tools

import (
	"context"
	"encoding/json"
	"strconv"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

const getBudgetToolName = "get_budget_info"

type CategorySpend struct {
	Name       string  `json:"name"`
	TotalSpend float64 `json:"totalSpend"`
}

type BudgetInfo struct {
	Categories []CategorySpend `json:"categories"`
	PayeeNames []string        `json:"payeeNames"`
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
				"budgetId": {
					Type:        "string",
					Description: "The id of the budget for which to fetch the budget info for.",
				},
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
			Required: []string{"budgetId", "dateRange"},
		},
	}
}

func (t GetBudgetInfoTool) Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	var args struct {
		BudgetID  string `json:"budgetId"`
		DateRange struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"dateRange"`
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse get_budget_info arguments", err)
	}
	if args.DateRange.Start == "" || args.DateRange.End == "" {
		return nil, errs.New(errs.CodeToolExecuteFail, "date range is required")
	}
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
	`, args.BudgetID, args.DateRange.Start, args.DateRange.End)
	if err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to execute tool get_budget_info", err)
	}
	defer categoryRows.Close()

	var categories []CategorySpend
	for categoryRows.Next() {
		var cs CategorySpend
		if err := categoryRows.Scan(&cs.Name, &cs.TotalSpend); err != nil {
			return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to scan category row", err)
		}
		categories = append(categories, cs)
	}
	if err := categoryRows.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to scan category rows", err)
	}

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
	`, args.BudgetID, args.DateRange.Start, args.DateRange.End)
	if err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to execute tool get_budget_info", err)
	}
	defer payeeRows.Close()

	var payeeNames []string
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

	return jsonToolResult(call, getBudgetToolName, BudgetInfo{
		Categories: categories,
		PayeeNames: payeeNames,
	})
}

func (t GetBudgetInfoTool) GetNormalizedName(isDone bool) string {
	if (isDone) {
		return "Loaded budget context"
	}
	return "Loading budget context..."
}

func (t GetBudgetInfoTool) Normalize(call sharedModel.ToolCall, result json.RawMessage) (*sharedModel.ToolResultNormalized, error) {
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
