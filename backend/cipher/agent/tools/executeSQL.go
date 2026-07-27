package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	executeSQLToolName = "execute_sql"
	executeSQLRowCap   = 500
	// executeSQLByteCap bounds the serialized result. The row cap alone is not
	// enough: 500 wide rows can be hundreds of KB of JSON, which crowds the
	// context window and makes every later turn more expensive.
	executeSQLByteCap = 128 * 1024
)

type ExecuteSQLTool struct {
	db.BaseRepository
	pool *pgxpool.Pool
}

type executeSQLArgs struct {
	Query  string `json:"query"`
	Reason string `json:"reason"`
}

func NewExecuteSQLTool(pool *pgxpool.Pool) Tool {
	return ExecuteSQLTool{BaseRepository: db.NewBaseRepository(pool), pool: pool}
}

func (t ExecuteSQLTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name:        executeSQLToolName,
		Description: "Fallback for questions no specific tool covers. Prefer get_spending_summary for spending totals grouped by category, payee, or tag; get_top_transactions for individual transactions; and get_budget_info to discover which categories, payees, and tags exist. Reach for this only when none of those can answer the question — for example category balances, account balances, month-over-month comparisons, or loan data. Call get_schema first unless its output is already in this conversation, and follow the query_rules it returns. The query must be a single SELECT or WITH statement scoped by budget_id.",
		InputSchema: sharedModel.ToolSchema{
			Type: "object",
			Properties: map[string]sharedModel.ToolSchema{
				"query": {
					Type:        "string",
					Description: "A single read-only PostgreSQL SELECT query. Do not use INSERT, UPDATE, DELETE, DROP, ALTER, TRUNCATE, CREATE, or multiple statements.",
				},
				"reason": {
					Type:        "string",
					Description: "Brief explanation of why this query is needed and what user question it answers.",
				},
			},
			Required:             []string{"query", "reason"},
			AdditionalProperties: false,
		},
	}
}

func (t ExecuteSQLTool) Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	var args executeSQLArgs
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse execute_sql arguments", err)
	}

	query, err := validateReadOnlyQuery(args.Query)
	if err != nil {
		return nil, err
	}

	budgetID := utils.MustBudgetID(ctx)
	results := make([]map[string]any, 0)

	// Postgres, not the validator above, is what actually enforces read-only
	// access and budget isolation here — see withBudgetScopedTx.
	err = withBudgetScopedTx(ctx, t.pool, budgetID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		fields := rows.FieldDescriptions()
		values := make([]any, len(fields))
		valuePtrs := make([]any, len(fields))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		approxBytes := 0
		for rows.Next() {
			if len(results) >= executeSQLRowCap || approxBytes >= executeSQLByteCap {
				break
			}
			if err := rows.Scan(valuePtrs...); err != nil {
				return err
			}
			row := map[string]any{}
			for i, field := range fields {
				value := normalizeSQLValue(values[i])
				row[string(field.Name)] = value
				approxBytes += len(field.Name) + approxValueSize(value)
			}
			results = append(results, row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}

	return jsonToolResult(call, call.Name, results)
}

// approxValueSize is a cheap size estimate for the byte cap. It does not need to
// be exact — it only has to stop an unbounded result from reaching the model.
func approxValueSize(value any) int {
	switch v := value.(type) {
	case nil:
		return 4
	case string:
		return len(v)
	default:
		return 16
	}
}

func validateReadOnlyQuery(query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", errs.New(errs.CodeInternalError, "execute_sql query is empty")
	}

	query = strings.TrimSuffix(query, ";")
	if strings.Contains(query, ";") {
		return "", errs.New(errs.CodeInternalError, "execute_sql accepts only one statement")
	}

	// Structural check only — a fast, obvious rejection with a clear message the
	// model can act on. It is deliberately NOT the security boundary: the old
	// keyword blocklist here was bypassable (UPDATE\r\n, DELETE(, a data-modifying
	// CTE) and produced false positives on any query containing a blocked word in
	// a string literal. Writes are now rejected by the read-only transaction in
	// withBudgetScopedTx, and cross-budget reads by row-level security.
	queryParts := strings.Fields(strings.ToUpper(query))
	if len(queryParts) == 0 || (queryParts[0] != "SELECT" && queryParts[0] != "WITH") {
		return "", errs.New(errs.CodeInvalidArgument, "execute_sql accepts only SELECT or WITH queries")
	}

	return query, nil
}

func normalizeSQLValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case pgtype.Numeric:
		floatValue, err := v.Float64Value()
		if err == nil && floatValue.Valid {
			return floatValue.Float64
		}
		return fmt.Sprintf("%v", v)
	default:
		return v
	}
}

func (t ExecuteSQLTool) GetNormalizedName(isDone bool) string {
	if isDone {
		return "Queried data"
	}
	return "Querying data..."
}

func (t ExecuteSQLTool) Normalize(
	call sharedModel.ToolCall,
	result json.RawMessage,
) (*sharedModel.ToolResultNormalized, error) {
	var args executeSQLArgs
	if len(call.Arguments) > 0 {
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, "parse execute_sql normalized arguments", err)
		}
	}

	var toolResult sharedModel.ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse execute_sql normalized result", err)
	}

	var rows []map[string]any
	for _, block := range toolResult.Content {
		if block.Type != "text" || strings.TrimSpace(block.Text) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(block.Text), &rows); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, "parse execute_sql result rows", err)
		}
		break
	}

	rowCount := len(rows)
	summary := fmt.Sprintf("Returned %d rows", rowCount)
	if rowCount == 1 {
		summary = "Returned 1 row"
	}
	if rowCount >= executeSQLRowCap {
		summary = fmt.Sprintf("Returned first %d rows", executeSQLRowCap)
	}

	normalized := map[string]any{
		"rowCount": rowCount,
	}
	if args.Reason != "" {
		normalized["reason"] = args.Reason
	}
	if rowCount >= executeSQLRowCap {
		normalized["truncated"] = true
		normalized["rowCap"] = executeSQLRowCap
	}

	normalizedJSON, err := json.Marshal(normalized)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "marshal execute_sql normalized result", err)
	}

	return &sharedModel.ToolResultNormalized{
		DisplayName: t.GetNormalizedName(true),
		Summary:     summary,
		Result:      json.RawMessage(normalizedJSON),
	}, nil
}
