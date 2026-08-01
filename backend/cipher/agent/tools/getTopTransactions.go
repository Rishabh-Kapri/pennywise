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
	topTransactionsToolName     = "get_top_transactions"
	topTransactionsDefaultLimit = 10
	topTransactionsMaxLimit     = 50
)

const (
	directionSpending = "spending"
	directionIncome   = "income"
)

type GetTopTransactionsTool struct {
	db *pgxpool.Pool
}

type topTransactionsArgs struct {
	DateRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"dateRange"`
	Limit        int    `json:"limit"`
	Direction    string `json:"direction"`
	CategoryName string `json:"categoryName"`
	PayeeName    string `json:"payeeName"`
	TagName      string `json:"tagName"`
}

type topTransactionRow struct {
	Date         string   `json:"date"`
	Amount       float64  `json:"amount"`
	PayeeName    string   `json:"payeeName,omitempty"`
	CategoryName string   `json:"categoryName,omitempty"`
	AccountName  string   `json:"accountName,omitempty"`
	TagNames     []string `json:"tagNames,omitempty"`
	Note         string   `json:"note,omitempty"`
}

type topTransactionsResult struct {
	DateRange    map[string]string   `json:"dateRange"`
	Direction    string              `json:"direction"`
	Transactions []topTransactionRow `json:"transactions"`
	Truncated    bool                `json:"truncated,omitempty"`
}

func NewGetTopTransactionsTool(db *pgxpool.Pool) Tool {
	return GetTopTransactionsTool{db: db}
}

func (t GetTopTransactionsTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name: topTransactionsToolName,
		Description: "Return the largest individual transactions in a date range, optionally filtered by " +
			"category, payee, or tag. Prefer this over execute_sql for 'what was my biggest purchase', " +
			"'show me my largest transactions', or when the user wants examples behind a total. " +
			"Returns a bounded number of rows, never full history.",
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
				"direction": {
					Type:        "string",
					Enum:        &[]any{directionSpending, directionIncome},
					Description: "Whether to return spending or income. Defaults to spending.",
				},
				"limit": {
					Type: "integer",
					Description: fmt.Sprintf(
						"Maximum transactions to return, largest first. Default %d, maximum %d.",
						topTransactionsDefaultLimit, topTransactionsMaxLimit,
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
					Type:        "string",
					Description: "Optional tag filter, matched case-insensitively as a partial name.",
				},
			},
			Required:             []string{"dateRange"},
			AdditionalProperties: false,
		},
	}
}

// Filters are applied as fixed predicates with bind parameters; an empty filter
// short-circuits via the "$n = ”" test rather than being concatenated in.
// Amounts stay signed in the output so the model can tell spend from income,
// but ordering is by magnitude.
const topTransactionsQuery = `
	SELECT t.date,
	       t.amount,
	       COALESCE(p.name, '')  AS payee_name,
	       COALESCE(c.name, '')  AS category_name,
	       COALESCE(a.name, '')  AS account_name,
	       COALESCE(t.note, '')  AS note,
	       COALESCE(
	         ARRAY(
	           SELECT tg.name FROM tags tg
	           WHERE tg.id = ANY(t.tag_ids) AND tg.budget_id = t.budget_id AND tg.deleted = FALSE
	           ORDER BY tg.name
	         ),
	         ARRAY[]::text[]
	       ) AS tag_names
	FROM transactions t
	LEFT JOIN payees p     ON p.id = t.payee_id     AND p.budget_id = t.budget_id
	LEFT JOIN categories c ON c.id = t.category_id  AND c.budget_id = t.budget_id
	LEFT JOIN accounts a   ON a.id = t.account_id   AND a.budget_id = t.budget_id
	WHERE t.budget_id = $1
	  AND t.deleted = FALSE
	  AND t.date >= $2
	  AND t.date <= $3
	  AND t.transfer_account_id IS NULL
	  AND CASE WHEN $4 = 'income' THEN t.amount > 0 ELSE t.amount < 0 END
	  AND ($5 = '' OR c.name ILIKE '%' || $5 || '%')
	  AND ($6 = '' OR p.name ILIKE '%' || $6 || '%')
	  AND ($7 = '' OR EXISTS (
	        SELECT 1 FROM tags tg
	        WHERE tg.id = ANY(t.tag_ids)
	          AND tg.budget_id = t.budget_id
	          AND tg.deleted = FALSE
	          AND tg.name ILIKE '%' || $7 || '%'
	      ))
	ORDER BY ABS(t.amount) DESC, t.date DESC
	LIMIT $8`

func (t GetTopTransactionsTool) Execute(
	ctx context.Context,
	call sharedModel.ToolCall,
) (*sharedModel.ToolResult, error) {
	var args topTransactionsArgs
	if err := decodeToolArgs(topTransactionsToolName, call.Arguments, &args); err != nil {
		return nil, err
	}

	if args.DateRange.Start == "" || args.DateRange.End == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "get_top_transactions requires dateRange.start and dateRange.end")
	}

	direction := args.Direction
	if direction == "" {
		direction = directionSpending
	}
	if direction != directionSpending && direction != directionIncome {
		return nil, errs.New(
			errs.CodeInvalidArgument,
			"get_top_transactions direction must be one of spending, income; got %q",
			direction,
		)
	}

	limit := args.Limit
	if limit <= 0 {
		limit = topTransactionsDefaultLimit
	}
	if limit > topTransactionsMaxLimit {
		limit = topTransactionsMaxLimit
	}

	budgetID := utils.MustBudgetID(ctx)
	result := topTransactionsResult{
		DateRange: map[string]string{
			"start": args.DateRange.Start,
			"end":   args.DateRange.End,
		},
		Direction:    direction,
		Transactions: make([]topTransactionRow, 0, limit),
	}

	err := withBudgetScopedTx(ctx, t.db, budgetID, func(tx pgx.Tx) error {
		rows, err := tx.Query(
			ctx, topTransactionsQuery,
			budgetID,
			args.DateRange.Start,
			args.DateRange.End,
			direction,
			strings.TrimSpace(args.CategoryName),
			strings.TrimSpace(args.PayeeName),
			strings.TrimSpace(args.TagName),
			limit,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var row topTransactionRow
			if err := rows.Scan(
				&row.Date,
				&row.Amount,
				&row.PayeeName,
				&row.CategoryName,
				&row.AccountName,
				&row.Note,
				&row.TagNames,
			); err != nil {
				return err
			}
			result.Transactions = append(result.Transactions, row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeToolExecuteFail, "failed to execute get_top_transactions", err)
	}

	result.Truncated = len(result.Transactions) >= limit

	return jsonToolResult(call, topTransactionsToolName, result)
}

func (t GetTopTransactionsTool) GetNormalizedName(isDone bool) string {
	if isDone {
		return "Found transactions"
	}
	return "Finding transactions..."
}

func (t GetTopTransactionsTool) Normalize(
	call sharedModel.ToolCall,
	result json.RawMessage,
) (*sharedModel.ToolResultNormalized, error) {
	var toolResult sharedModel.ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse get_top_transactions normalized result", err)
	}

	var transactions topTransactionsResult
	for _, block := range toolResult.Content {
		if block.Type != "text" || strings.TrimSpace(block.Text) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(block.Text), &transactions); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, "parse get_top_transactions result", err)
		}
		break
	}

	count := len(transactions.Transactions)
	normalized := map[string]any{
		"transactionCount": count,
		"direction":        transactions.Direction,
		"dateRange":        transactions.DateRange,
	}
	normalizedJSON, err := json.Marshal(normalized)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "marshal get_top_transactions normalized result", err)
	}

	return &sharedModel.ToolResultNormalized{
		DisplayName: t.GetNormalizedName(true),
		Summary:     "Found " + pluralizeCount(count, "transaction", "transactions"),
		Result:      json.RawMessage(normalizedJSON),
	}, nil
}
