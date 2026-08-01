package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
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
	// Filters mirror get_top_transactions exactly. Keep the two vocabularies
	// identical: a name accepted by one tool and silently dropped by the other is
	// how a filtered question comes back with unfiltered numbers.
	CategoryName string `json:"categoryName"`
	PayeeName    string `json:"payeeName"`
	TagName      string `json:"tagName"`
}

type spendingSummaryRow struct {
	Name             string  `json:"name"`
	Total            float64 `json:"total"`
	TransactionCount int     `json:"transactionCount"`
}

type spendingSummaryResult struct {
	GroupBy   string            `json:"groupBy"`
	DateRange map[string]string `json:"dateRange"`
	// Filters echoes back what was actually applied, so the model states the
	// scope of a number rather than inferring it from what it asked for.
	Filters   map[string]string    `json:"filters,omitempty"`
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
			"optionally filtered by category, payee, or tag, ordered by amount spent. " +
			"Prefer this over execute_sql for any 'how much did I spend on X' " +
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
				"categoryName": {
					Type:        "string",
					Description: "Optional category filter, matched case-insensitively as a partial name.",
				},
				"payeeName": {
					Type:        "string",
					Description: "Optional payee filter, matched case-insensitively as a partial name.",
				},
				"tagName": {
					Type: "string",
					Description: "Optional tag filter, matched case-insensitively as a partial name. " +
						"Use this to restrict a summary to one tag, e.g. spending by category within a trip tag; " +
						"groupBy \"tag\" instead breaks the range down across all tags.",
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
// spendingScope is everything that decides which transactions count. It is
// applied identically to the grouping queries and to the total, because a total
// computed over a wider set than the rows beneath it is worse than no total at
// all — the model reports a filtered breakdown against an unfiltered
// denominator and every percentage it derives is wrong.
type spendingScope struct {
	budgetID uuid.UUID
	start    string
	end      string
	filters  entityFilters
}

// applySpendingScope adds the predicates common to every spending query.
// Spending is stored as a negative amount; transfers carry a transfer_account_id
// and are excluded because moving money between your own accounts is not
// spending.
func applySpendingScope(query sq.SelectBuilder, scope spendingScope) sq.SelectBuilder {
	query = query.
		Where(sq.Eq{"t.budget_id": scope.budgetID}).
		Where(sq.Eq{"t.deleted": false}).
		Where(sq.GtOrEq{"t.date": scope.start}).
		Where(sq.LtOrEq{"t.date": scope.end}).
		Where(sq.Lt{"t.amount": 0}).
		Where(sq.Eq{"t.transfer_account_id": nil})

	return scope.filters.apply(query)
}

// spendingSummaryGroupQuery builds the grouped breakdown. The grouping dimension
// selects a fixed builder — it is never interpolated into SQL.
func spendingSummaryGroupQuery(groupBy string, scope spendingScope, limit uint64) (sq.SelectBuilder, error) {
	switch groupBy {
	case groupByCategory:
		query := psql.
			Select(
				"COALESCE(c.name, 'Uncategorized') AS name",
				"-SUM(t.amount) AS total",
				"COUNT(*) AS transaction_count",
			).
			From("transactions t").
			LeftJoin("categories c ON c.id = t.category_id AND c.budget_id = t.budget_id").
			GroupBy("COALESCE(c.name, 'Uncategorized')")
		return applySpendingScope(query, scope).
			Where(sq.Expr("COALESCE(c.is_system, FALSE) = FALSE")).
			OrderBy("total DESC").
			Limit(limit), nil

	case groupByPayee:
		query := psql.
			Select(
				"COALESCE(p.name, 'Unknown payee') AS name",
				"-SUM(t.amount) AS total",
				"COUNT(*) AS transaction_count",
			).
			From("transactions t").
			LeftJoin("payees p ON p.id = t.payee_id AND p.budget_id = t.budget_id").
			GroupBy("COALESCE(p.name, 'Unknown payee')")
		return applySpendingScope(query, scope).
			OrderBy("total DESC").
			Limit(limit), nil

	// tag_ids is a UUID[] column on transactions, not a join table.
	case groupByTag:
		query := psql.
			Select(
				"tg.name AS name",
				"-SUM(t.amount) AS total",
				"COUNT(*) AS transaction_count",
			).
			From("transactions t").
			Join("tags tg ON tg.id = ANY(t.tag_ids) AND tg.budget_id = t.budget_id AND tg.deleted = FALSE").
			GroupBy("tg.name")
		return applySpendingScope(query, scope).
			OrderBy("total DESC").
			Limit(limit), nil

	default:
		return sq.SelectBuilder{}, errs.New(
			errs.CodeInvalidArgument,
			"get_spending_summary groupBy must be one of category, payee, tag; got %q",
			groupBy,
		)
	}
}

// spendingSummaryTotalQuery is the true total for the range and filters. It is
// computed separately from the grouped rows so a truncating limit cannot make
// the reported total wrong, and so tag grouping (where one transaction can
// appear under several tags) still has an unambiguous overall figure.
func spendingSummaryTotalQuery(scope spendingScope) sq.SelectBuilder {
	query := psql.
		Select("COALESCE(-SUM(t.amount), 0)").
		From("transactions t").
		LeftJoin("categories c ON c.id = t.category_id AND c.budget_id = t.budget_id")

	return applySpendingScope(query, scope).
		Where(sq.Expr("COALESCE(c.is_system, FALSE) = FALSE"))
}

func (t GetSpendingSummaryTool) Execute(
	ctx context.Context,
	call sharedModel.ToolCall,
) (*sharedModel.ToolResult, error) {
	var args spendingSummaryArgs
	if err := decodeToolArgs(spendingSummaryToolName, call.Arguments, &args); err != nil {
		return nil, err
	}

	if args.DateRange.Start == "" || args.DateRange.End == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "get_spending_summary requires dateRange.start and dateRange.end")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = spendingSummaryDefaultLimit
	}
	if limit > spendingSummaryMaxLimit {
		limit = spendingSummaryMaxLimit
	}

	budgetID := utils.MustBudgetID(ctx)
	scope := spendingScope{
		budgetID: budgetID,
		start:    args.DateRange.Start,
		end:      args.DateRange.End,
		filters:  newEntityFilters(args.CategoryName, args.PayeeName, args.TagName),
	}

	groupQuery, err := spendingSummaryGroupQuery(args.GroupBy, scope, uint64(limit))
	if err != nil {
		return nil, err
	}

	groupSQL, groupArgs, err := groupQuery.ToSql()
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "build get_spending_summary query", err)
	}
	totalSQL, totalArgs, err := spendingSummaryTotalQuery(scope).ToSql()
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "build get_spending_summary total query", err)
	}

	result := spendingSummaryResult{
		GroupBy: args.GroupBy,
		DateRange: map[string]string{
			"start": args.DateRange.Start,
			"end":   args.DateRange.End,
		},
		Filters: scope.filters.applied(),
		Rows:    make([]spendingSummaryRow, 0, limit),
	}

	err = withBudgetScopedTx(ctx, t.db, budgetID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, totalSQL, totalArgs...).Scan(&result.Total); err != nil {
			return err
		}

		rows, err := tx.Query(ctx, groupSQL, groupArgs...)
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
	if len(result.Rows) == 0 && scope.filters.any() {
		// "No spending" and "the filter matched nothing" look identical in an
		// empty result set. Say which, so the model reports an unmatched tag as
		// an unmatched tag instead of as zero spend.
		result.Note = strings.TrimSpace(result.Note +
			" No transactions matched the requested filter; the filter names may not exist in this budget. Use get_budget_info to list the real category, payee, and tag names.")
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
	if len(summary.Filters) > 0 {
		normalized["filters"] = summary.Filters
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
