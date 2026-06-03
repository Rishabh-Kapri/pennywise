package tools

import (
	"context"
	"encoding/json"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

const getSchemaToolName = "get_schema"

type GetSchemaTool struct{}

type getSchemaArgs struct {
	Tables []string `json:"tables"`
}

type tableSchema struct {
	Columns []string `json:"columns"`
	Notes   []string `json:"notes,omitempty"`
}

type schemaResult struct {
	DomainContext []string               `json:"domain_context"`
	Tables        map[string]tableSchema `json:"tables"`
	QueryRules    []string               `json:"query_rules"`
	ExampleQuery  string                 `json:"example_query"`
}

func NewGetSchemaTool() Tool {
	return GetSchemaTool{}
}

func (t GetSchemaTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name:        getSchemaToolName,
		Description: "Return Pennywise database schema and SQL rules for read-only transaction, account, category, payee, and category balance queries. Call this before execute_sql unless the schema was already returned in the current conversation.",
		InputSchema: sharedModel.ToolSchema{
			Type: "object",
			Properties: map[string]sharedModel.ToolSchema{
				"tables": {
					Type:        "array",
					Description: "Optional list of tables to return. Supported values: transactions, accounts, categories, payees, category_balances_by_month. Omit to return all supported schemas.",
					Items: &sharedModel.ToolSchema{
						Type: "string",
					},
				},
			},
			Required:             []string{},
			AdditionalProperties: false,
		},
	}
}

func (t GetSchemaTool) Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	requested := map[string]bool{}
	if len(call.Arguments) > 0 {
		var args getSchemaArgs
		if err := json.Unmarshal(call.Arguments, &args); err == nil {
			for _, table := range args.Tables {
				requested[table] = true
			}
		}
	}

	allTables := map[string]tableSchema{
		"accounts": {
			Columns: []string{
				"id UUID PRIMARY KEY",
				"name TEXT NOT NULL",
				"budget_id UUID NOT NULL REFERENCES budgets(id)",
				"type TEXT NOT NULL",
				"closed BOOLEAN DEFAULT false",
			},
		},
		"transactions": {
			Columns: []string{
				"id UUID PRIMARY KEY",
				"budget_id UUID NOT NULL REFERENCES budgets(id)",
				"date TEXT NOT NULL",
				"payee_id UUID REFERENCES payees(id)",
				"category_id UUID REFERENCES categories(id)",
				"account_id UUID NOT NULL REFERENCES accounts(id)",
				"amount NUMERIC(12, 2) NOT NULL",
			},
			Notes: []string{
				"amount: negative = spending, positive = income. For spend totals: COALESCE(-SUM(amount),0) WHERE amount < 0.",
				"date is TEXT YYYY-MM-DD, cast with date::date for comparisons.",
				"Always filter deleted = FALSE.",
			},
		},
		"categories": {
			Columns: []string{
				"id UUID PRIMARY KEY",
				"name TEXT NOT NULL",
				"budget_id UUID NOT NULL REFERENCES budgets(id)",
				"category_group_id UUID NOT NULL REFERENCES category_groups(id)",
			},
		},
		"payees": {
			Columns: []string{
				"id UUID PRIMARY KEY",
				"name TEXT NOT NULL",
				"budget_id UUID NOT NULL REFERENCES budgets(id)",
			},
		},
		"category_balances_by_month": {
			Columns: []string{
				"category_id UUID",
				"category_name TEXT",
				"budget_id UUID",
				"month TEXT NOT NULL",
				"available_balance NUMERIC(12, 2)",
				"budgeted NUMERIC(12, 2)",
				"monthly_activity NUMERIC(12, 2)",
			},
			Notes: []string{
				"month is YYYY-MM (not YYYY-MM-DD). Filter: month = '2026-05'.",
				"Use available_balance directly; do not recalculate it.",
				"For month comparisons use TO_DATE(month,'YYYY-MM').",
			},
		},
	}

	tables := make(map[string]tableSchema)
	for name, schema := range allTables {
		if len(requested) == 0 || requested[name] {
			tables[name] = schema
		}
	}

	return jsonToolResult(call, getSchemaToolName, schemaResult{
		Tables: tables,
	})
}

func (t GetSchemaTool) GetNormalizedName(isDone bool) string {
	return ""
}

func (t GetSchemaTool) Normalize(
	call sharedModel.ToolCall,
	result json.RawMessage,
) (*sharedModel.ToolResultNormalized, error) {
	// this tool will not return any result, won't be shown in the ui
	return nil, nil
}
