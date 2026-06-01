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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	executeSQLToolName = "execute_sql"
	executeSQLRowCap   = 500
)

type ExecuteSQLTool struct {
	db.BaseRepository
}

type executeSQLArgs struct {
	Query  string `json:"query"`
	Reason string `json:"reason"`
}

func NewExecuteSQLTool(pool *pgxpool.Pool) Tool {
	return ExecuteSQLTool{BaseRepository: db.NewBaseRepository(pool)}
}


func (t ExecuteSQLTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name:        executeSQLToolName,
		Description: "Execute a read-only SQL query against the Pennywise database. Use this only when the user asks for budget, transaction, account, category, payee, tag, or loan data that is not available from another more specific tool. Call get_schema before this tool unless schema was already returned in the current conversation. The query must be a single SELECT statement, must include budget scoping when querying budget-owned data, must follow get_schema query rules, and must not infer account/category/payee name matches when context IDs are available.",
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

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := t.Executor(nil).Query(queryCtx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	results := make([]map[string]any, 0)
	values := make([]any, len(fields))
	valuePtrs := make([]any, len(fields))

	for i := range values {
		valuePtrs[i] = &values[i]
	}
	rowCount := 0
	for rows.Next() {
		if rowCount >= executeSQLRowCap {
			break
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, field := range fields {
			row[string(field.Name)] = normalizeSQLValue(values[i])
		}
		results = append(results, row)
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jsonToolResult(call, call.Name, results)
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

	upperQuery := strings.ToUpper(query)
	queryParts := strings.Fields(upperQuery)
	if len(queryParts) == 0 || (queryParts[0] != "SELECT" && queryParts[0] != "WITH") {
		return "", errs.New(errs.CodeInternalError, "execute_sql accepts only SELECT or WITH queries")
	}

	blockedTerms := []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE", "CREATE", "GRANT", "REVOKE"}
	for _, term := range blockedTerms {
		if strings.Contains(upperQuery, term+" ") || strings.Contains(upperQuery, term+"\n") ||
			strings.Contains(upperQuery, term+"\t") {
			return "", errs.New(errs.CodeInternalError, "execute_sql rejected non-read-only keyword: %s", term)
		}
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
	if (isDone) {
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
