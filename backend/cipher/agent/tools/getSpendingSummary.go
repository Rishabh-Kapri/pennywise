package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	spendingSummaryToolName     = "get_spending_summary"
	spendingSummaryDefaultLimit = 25
	spendingSummaryMaxLimit     = 100
)

// Grouping dimensions. The query for each is a fixed string chosen by this
// value — the dimension is never interpolated into SQL.
const (
	groupByCategory = "category"
	groupByPayee    = "payee"
	groupByTag      = "tag"
)

type GetSpendingSummaryTool struct {
	db *pgxpool.Pool
}

type spendingSummaryArgs struct {
	DateRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"dateRange"`
	GroupBy string `json:"groupBy"`
	Limit   int    `json:"limit"`
}

type spendingSummaryRow struct {
	Name             string  `json:"name"`
	Total            float64 `json:"total"`
	TransactionCount int     `json:"transactionCount"`
}

type spendingSummaryResult struct {
	GroupBy   string               `json:"groupBy"`
	DateRange map[string]string    `json:"dateRange"`
	Total     float64              `json:"total"`
	Rows      []spendingSummaryRow `json:"rows"`
	Truncated bool                 `json:"truncated,omitempty"`
	Note      string               `json:"note,omitempty"`
}

func NewGetSpendingSummaryTool(db *pgxpool.Pool) Tool {
	return GetSpendingSummaryTool{db: db}
}

func (t GetSpendingSummaryTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name: spendingSummaryToolName,
		Description: "Return total spending in a date range, grouped by category, payee, or tag, " +
			"ordered by amount spent. Prefer this over execute_sql for any 'how much did I spend on X' " +
			"or 'what did I spend the most on' question. Amounts are positive numbers representing money spent. " +
			"Transfers between accounts and income are excluded.",
		InputSchema: sharedModel.ToolSchema{
			Type: "object",
			Properties: map[string]sharedModel.ToolSchema{
				"dateRange": {
					Type: "object",
					Properties: map[string]sharedModel.ToolSchema{
						"start": {Type: "string", Description: "Inclusive start date, YYYY-MM-DD."},
						"end":   {Type: "string", Description: "Inclusive end date, YYYY-MM-DD."},
					},
					Required: []string{"start", "end"},
				},
				"groupBy": {
					Type:        "string",
					Enum:        &[]any{groupByCategory, groupByPayee, groupByTag},
					Description: "Dimension to group spending by.",
				},
				"limit": {
					Type: "integer",
					Description: fmt.Sprintf(
						"Maximum groups to return, ordered by amount spent descending. Default %d, maximum %d.",
						spendingSummaryDefaultLimit, spendingSummaryMaxLimit,
					),
				},
			},
			Required:             []string{"dateRange", "groupBy"},
			AdditionalProperties: false,
		},
	}
}

// Spending is stored as a negative amount, so totals are negated to read as
// positive "money spent". Transfers carry a transfer_account_id and are excluded
// because moving money between your own accounts is not spending.
const spendingSummaryBaseFilter = `
	  AND t.deleted = FALSE
	  AND t.date >= $2
	  AND t.date <= $3
	  AND t.amount < 0
	  AND t.transfer_account_id IS NULL`

var spendingSummaryQueries = map[string]string{
	groupByCategory: `
		SELECT COALESCE(c.name, 'Uncategorized') AS name,
		       -SUM(t.amount) AS total,
		       COUNT(*) AS transaction_count
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id AND c.budget_id = t.budget_id
		WHERE t.budget_id = $1` + spendingSummaryBaseFilter + `
		  AND COALESCE(c.is_system, FALSE) = FALSE
		GROUP BY COALESCE(c.name, 'Uncategorized')
		ORDER BY total DESC
		LIMIT $4`,

	groupByPayee: `
		SELECT COALESCE(p.name, 'Unknown payee') AS name,
		       -SUM(t.amount) AS total,
		       COUNT(*) AS transaction_count
		FROM transactions t
		LEFT JOIN payees p ON p.id = t.payee_id AND p.budget_id = t.budget_id
		WHERE t.budget_id = $1` + spendingSummaryBaseFilter + `
		GROUP BY COALESCE(p.name, 'Unknown payee')
		ORDER BY total DESC
		LIMIT $4`,

	// tag_ids is a UUID[] column on transactions, not a join table.
	groupByTag: `
		SELECT tg.name AS name,
		       -SUM(t.amount) AS total,
		       COUNT(*) AS transaction_count
		FROM transactions t
		JOIN tags tg ON tg.id = ANY(t.tag_ids) AND tg.budget_id = t.budget_id AND tg.deleted = FALSE
		WHERE t.budget_id = $1` + spendingSummaryBaseFilter + `
		GROUP BY tg.name
		ORDER BY total DESC
		LIMIT $4`,
}

// spendingSummaryTotalQuery is the true total for the range. It is computed
// separately so a truncating limit cannot make the reported total wrong, and so
// tag grouping (where one transaction can appear under several tags) still has
// an unambiguous overall figure.
const spendingSummaryTotalQuery = `
	SELECT COALESCE(-SUM(t.amount), 0)
	FROM transactions t
	LEFT JOIN categories c ON c.id = t.category_id AND c.budget_id = t.budget_id
	WHERE t.budget_id = $1` + spendingSummaryBaseFilter + `
	  AND COALESCE(c.is_system, FALSE) = FALSE`

func (t GetSpendingSummaryTool) Execute(
	ctx context.Context,
	call sharedModel.ToolCall,
) (*sharedModel.ToolResult, error) {
	var args spendingSummaryArgs
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse get_spending_summary arguments", err)
	}

	if args.DateRange.Start == "" || args.DateRange.End == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "get_spending_summary requires dateRange.start and dateRange.end")
	}

	query, ok := spendingSummaryQueries[args.GroupBy]
	if !ok {
		return nil, errs.New(
			errs.CodeInvalidArgument,
			"get_spending_summary groupBy must be one of category, payee, tag; got %q",
			args.GroupBy,
		)
	}

	limit := args.Limit
	if limit <= 0 {
		limit = spendingSummaryDefaultLimit
	}
	if limit > spendingSummaryMaxLimit {
		limit = spendingSummaryMaxLimit
	}

	budgetID := utils.MustBudgetID(ctx)
	result := spendingSummaryResult{
		GroupBy: args.GroupBy,
		DateRange: map[string]string{
			"start": args.DateRange.Start,
			"end":   args.DateRange.End,
		},
		Rows: make([]spendingSummaryRow, 0, limit),
	}

	err := withBudgetScopedTx(ctx, t.db, budgetID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(
			ctx, spendingSummaryTotalQuery, budgetID, args.DateRange.Start, args.DateRange.End,
		).Scan(&result.Total); err != nil {
			return err
		}

		rows, err := tx.Query(ctx, query, budgetID, args.DateRange.Start, args.DateRange.End, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var row spendingSummaryRow
			if err := rows.Scan(&row.Name, &row.Total, &row.TransactionCount); err != nil {
				return err
			}
			result.Rows = append(result.Rows, row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to execute get_spending_summary", err)
	}

	if len(result.Rows) >= limit {
		result.Truncated = true
		result.Note = fmt.Sprintf("Showing the top %d groups only; total reflects all spending in the range.", limit)
	}
	if args.GroupBy == groupByTag {
		// A transaction can carry several tags, so per-tag totals legitimately sum
		// to more than the range total. Say so rather than let the model "correct"
		// the discrepancy.
		result.Note = strings.TrimSpace(result.Note +
			" A transaction with multiple tags is counted under each of them, so per-tag totals can add up to more than the overall total.")
	}

	return jsonToolResult(call, spendingSummaryToolName, result)
}

func (t GetSpendingSummaryTool) GetNormalizedName(isDone bool) string {
	if isDone {
		return "Summarized spending"
	}
	return "Summarizing spending..."
}

func (t GetSpendingSummaryTool) Normalize(
	call sharedModel.ToolCall,
	result json.RawMessage,
) (*sharedModel.ToolResultNormalized, error) {
	var toolResult sharedModel.ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse get_spending_summary normalized result", err)
	}

	var summary spendingSummaryResult
	for _, block := range toolResult.Content {
		if block.Type != "text" || strings.TrimSpace(block.Text) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(block.Text), &summary); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, "parse get_spending_summary result", err)
		}
		break
	}

	normalized := map[string]any{
		"groupBy":    summary.GroupBy,
		"groupCount": len(summary.Rows),
		"dateRange":  summary.DateRange,
	}
	normalizedJSON, err := json.Marshal(normalized)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "marshal get_spending_summary normalized result", err)
	}

	return &sharedModel.ToolResultNormalized{
		DisplayName: t.GetNormalizedName(true),
		Summary:     fmt.Sprintf("Grouped by %s across %s", summary.GroupBy, pluralizeCount(len(summary.Rows), "group", "groups")),
		Result:      json.RawMessage(normalizedJSON),
	}, nil
}
