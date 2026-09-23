package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const getBudgetToolName = "get_budget_info"

// budgetInfoNameCap bounds each name list. An active budget accumulates hundreds
// of payees, and dumping all of them into context crowds out the actual answer.
const budgetInfoNameCap = 200

type BudgetInfo struct {
	Categories []string `json:"categories"`
	PayeeNames []string `json:"payeeNames"`
	TagNames   []string `json:"tagNames"`
}

type budgetNamesRow struct {
	Categories []string `json:"categories"`
	PayeeNames []string `json:"payeeNames"`
	TagNames   []string `json:"tagNames"`
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
		Description: "Return the category, payee, and tag names the user actually used in a specific date range. Use this to discover what entities exist before answering questions about them. Only call this tool when the date range is known.",
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

func (t GetBudgetInfoTool) budgetNamesQuery(
	budgetID uuid.UUID,
	start string,
	end string,
	limit uint64,
) sq.SelectBuilder {
	base := func(columns ...string) sq.SelectBuilder {
		return sq.
			Select(columns...).
			From("transactions t").
			Where(sq.Eq{"t.budget_id": budgetID}).
			Where(sq.GtOrEq{"t.date": start}).
			Where(sq.LtOrEq{"t.date": end}).
			Where(sq.Eq{"t.deleted": false})
	}

	categories := base("DISTINCT c.name").
		Join(`
			categories c
				ON c.id = t.category_id
				AND c.budget_id = t.budget_id
		`).
		Where(sq.Eq{
			"c.is_system": false,
			"c.hidden":    false,
			"c.deleted":   false,
		}).
		Where(sq.NotEq{"c.name": nil}).
		OrderBy("c.name").
		Limit(limit)

	payees := base("DISTINCT p.name").
		Join(`
			payees p
				ON p.id = t.payee_id
				AND p.budget_id = t.budget_id
		`).
		Where(sq.Eq{"p.deleted": false}).
		Where(sq.NotEq{"p.name": nil}).
		OrderBy("p.name").
		Limit(limit)

	tags := base("DISTINCT tg.name").
		Join(`
			tags tg
				ON tg.id = ANY(t.tag_ids)
				AND tg.budget_id = t.budget_id
		`).
		Where(sq.Eq{"tg.deleted": false}).
		Where(sq.NotEq{"tg.name": nil}).
		OrderBy("tg.name").
		Limit(limit)

	return psql.
		Select().
		Column(sq.Expr("ARRAY(?) AS categories", categories)).
		Column(sq.Expr("ARRAY(?) AS payees", payees)).
		Column(sq.Expr("ARRAY(?) AS tags", tags))
}

func (t GetBudgetInfoTool) Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	var args BudgetToolArgs
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse get_budget_info arguments", err)
	}
	if args.DateRange.Start == "" || args.DateRange.End == "" {
		return nil, errs.New(errs.CodeToolExecuteFail, "date range is required")
	}

	var limit uint64 = budgetInfoNameCap

	budgetID := utils.MustBudgetID(ctx)

	querySQL, queryArgs, err := t.budgetNamesQuery(budgetID, args.DateRange.Start, args.DateRange.End, limit).ToSql()
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "build budgetNamesQuery query", err)
	}

	var result []budgetNamesRow

	err = withBudgetScopedTx(ctx, t.db, budgetID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, querySQL, queryArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var row budgetNamesRow
			if err := rows.Scan(&row.Categories, &row.PayeeNames, &row.TagNames); err != nil {
				return err
			}
			result = append(result, row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to execute tool get_budget_info", err)
	}

	return jsonToolResult(call, getBudgetToolName, result)
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
	tagCount := len(budgetInfo.TagNames)

	found := make([]string, 0, 3)
	if categoryCount > 0 {
		found = append(found, pluralizeCount(categoryCount, "category", "categories"))
	}
	if payeeCount > 0 {
		found = append(found, pluralizeCount(payeeCount, "payee", "payees"))
	}
	if tagCount > 0 {
		found = append(found, pluralizeCount(tagCount, "tag", "tags"))
	}

	summary := "Loaded budget context"
	if len(found) > 0 {
		summary = "Found " + joinWithAnd(found)
	}

	normalized := map[string]any{
		"categoryCount": categoryCount,
		"payeeCount":    payeeCount,
		"tagCount":      tagCount,
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

// joinWithAnd renders a list as "a", "a and b", or "a, b and c".
func joinWithAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}
