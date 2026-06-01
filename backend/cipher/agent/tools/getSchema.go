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
				"transfer_payee_id UUID REFERENCES payees(id)",
				"type TEXT NOT NULL",
				"suffix TEXT",
				"closed BOOLEAN DEFAULT false",
				"deleted BOOLEAN DEFAULT false",
				"created_at TIMESTAMPTZ NOT NULL DEFAULT now()",
				"updated_at TIMESTAMPTZ NOT NULL DEFAULT now()",
			},
			Notes: []string{
				"Use accounts for account-level filters and grouping; transactions.account_id references accounts.id.",
				"Prefer filtering by account_id from context when available instead of joining by account name.",
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
				"note TEXT",
				"dedupe_hash VARCHAR(64)",
				"status transaction_status NOT NULL DEFAULT 'MANUAL'",
				"raw_bank_text TEXT",
				"transfer_account_id UUID REFERENCES accounts(id)",
				"transfer_transaction_id UUID REFERENCES transactions(id)",
				"tag_ids UUID[] DEFAULT '{}'",
				"deleted BOOLEAN DEFAULT false",
				"created_at TIMESTAMPTZ NOT NULL DEFAULT now()",
				"updated_at TIMESTAMPTZ NOT NULL DEFAULT now()",
			},
			Notes: []string{
				"amount is signed: spending/outflow is negative, income/inflow is positive.",
				"Incoming transactions are assigned to the inflow category before that money is budgeted into individual spending/saving categories.",
				"For spending totals, filter amount < 0 and return a positive number with COALESCE(-SUM(amount), 0).",
				"date is TEXT; cast with date::date for date comparisons.",
			},
		},
		"categories": {
			Columns: []string{
				"id UUID PRIMARY KEY",
				"name TEXT NOT NULL",
				"budget_id UUID NOT NULL REFERENCES budgets(id)",
				"category_group_id UUID NOT NULL REFERENCES category_groups(id)",
				"note TEXT",
				"hidden BOOLEAN DEFAULT false",
				"is_system BOOLEAN DEFAULT false",
				"deleted BOOLEAN DEFAULT false",
				"created_at TIMESTAMPTZ NOT NULL DEFAULT now()",
				"updated_at TIMESTAMPTZ NOT NULL DEFAULT now()",
			},
		},
		"payees": {
			Columns: []string{
				"id UUID PRIMARY KEY",
				"name TEXT NOT NULL",
				"budget_id UUID NOT NULL REFERENCES budgets(id)",
				"transfer_account_id UUID REFERENCES accounts(id)",
				"deleted BOOLEAN DEFAULT false",
				"created_at TIMESTAMPTZ NOT NULL DEFAULT now()",
				"updated_at TIMESTAMPTZ NOT NULL DEFAULT now()",
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
				"Use this view for category availability, budgeted amount, monthly activity, overspending, and move-money questions.",
				"budgeted is the amount assigned to that category for the month.",
				"monthly_activity is the category transaction activity for that month.",
				"available_balance is the category balance available to spend or move for that month.",
				"Use available_balance directly; do not recalculate available from carryover_balance, budgeted, and monthly_activity.",
				"month is TEXT in YYYY-MM format; derive current month from get_today by taking the first 7 characters, for example 2026-05.",
				"For month comparisons, use TO_DATE(month, 'YYYY-MM') instead of casting month::date.",
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
		DomainContext: []string{
			"Pennywise is zero-based budgeting software, similar to YNAB.",
			"Income/inflow transactions enter through the inflow category, then that money is budgeted to individual categories.",
			"Category monthly balance data lives in category_balances_by_month, keyed by budget_id, category_id, and month.",
			"For a category-month, category_balances_by_month.budgeted is the amount assigned that month and available_balance is the category balance available to spend or move.",
			"When answering what money is available to move, use category_balances_by_month.available_balance directly.",
			"Category names, payee names, and account names are separate concepts; do not infer an account from a category or payee name unless the user explicitly asks for an account-level query.",
		},
		Tables: tables,
		QueryRules: []string{
			"Only generate single SELECT statements.",
			"Always filter budget-owned tables by budget_id using the scoped budget_id from context.",
			"Always filter deleted rows with deleted = FALSE when querying transactions, accounts, categories, or payees.",
			"Prefer category_id, payee_id, and account_id from context over joining by name when IDs are available.",
			"When the user names a category, payee, or account and no matching ID is available from context, resolve the name with case-insensitive partial matching on the relevant name column; user terms may omit emoji prefixes, punctuation, or decorative text.",
			"If a user-provided name has one plausible partial match, use it; if it has multiple plausible matches, ask a clarifying question before querying totals.",
			"Do not combine category-based and account-based results with UNION unless the user explicitly asks to compare or combine categories and accounts.",
			"Use category_balances_by_month when answering questions about category available balance, budgeted amount, monthly activity, overspending, or money to move.",
			"For category available balance questions, use category_balances_by_month.available_balance directly; do not calculate it manually.",
			"category_balances_by_month.month is YYYY-MM, not YYYY-MM-DD; for current month from get_today date 2026-05-05, filter month = '2026-05'.",
			"For this month or other relative dates, use explicit date bounds derived from get_today instead of current_date.",
			"For spending totals, use amount < 0 and COALESCE(-SUM(amount), 0) so the result is positive.",
			"For income totals, use amount > 0 and COALESCE(SUM(amount), 0).",
		},
		ExampleQuery: `SELECT category_name, available_balance
FROM category_balances_by_month
WHERE budget_id = '<scoped_budget_id>'
  AND month = '2026-05'
  AND available_balance > 0
ORDER BY available_balance DESC;`,
	})
}

func (t GetSchemaTool) GetNormalizedName(isDone bool) string {
	return ""
}

func (t GetSchemaTool) Normalize(call sharedModel.ToolCall, result json.RawMessage) (*sharedModel.ToolResultNormalized, error) {
	// this tool will not return any result, won't be shown in the ui
	return nil, nil
}
