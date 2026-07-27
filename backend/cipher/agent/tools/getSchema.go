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
					Description: "Optional list of tables to return. Supported values: transactions, accounts, categories, category_groups, payees, tags, monthly_budgets, category_balances_by_month. Omit to return all supported schemas.",
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

	tables := make(map[string]tableSchema)
	for name, schema := range allTables {
		if len(requested) == 0 || requested[name] {
			tables[name] = schema
		}
	}

	return jsonToolResult(call, getSchemaToolName, schemaResult{
		DomainContext: domainContext,
		Tables:        tables,
		QueryRules:    queryRules,
		ExampleQuery:  exampleQuery,
	})
}

// domainContext, queryRules and exampleQuery were declared on schemaResult but
// never populated, so execute_sql's instruction to "follow get_schema query
// rules" pointed at nothing.
var domainContext = []string{
	"Pennywise is zero-based budgeting software, similar to YNAB.",
	"Income is assigned to an inflow category first (categories.is_system = true), then budgeted out to individual categories.",
	"A transaction with a non-null transfer_account_id is a movement between the user's own accounts, not spending or income.",
}

var queryRules = []string{
	"Always filter deleted = FALSE on transactions, categories, payees, accounts and tags.",
	"Always scope budget-owned tables by budget_id using the budget id from application context.",
	"amount is negative for spending and positive for income. For spend totals use COALESCE(-SUM(amount), 0) WHERE amount < 0.",
	"Exclude transfers from spending and income figures with transfer_account_id IS NULL.",
	"date is TEXT in YYYY-MM-DD form; compare as text for ranges, or cast with date::date for date arithmetic.",
	"category_balances_by_month.month is YYYY-MM, not YYYY-MM-DD. Filter as month = '2026-05'; for ordering or arithmetic use TO_DATE(month, 'YYYY-MM').",
	"available_balance is a stored running balance. Use it directly, never recalculate it, and treat NULL as 'never budgeted' rather than zero.",
	"Tags are a UUID[] column on transactions (tag_ids), not a join table. For one tag use tg.id = ANY(t.tag_ids); for any-of use t.tag_ids && ARRAY['<id>']::uuid[].",
	"Exclude the inflow category from spending breakdowns with COALESCE(c.is_system, FALSE) = FALSE.",
}

const exampleQuery = `-- Spending by category for one month, transfers and inflow excluded:
SELECT COALESCE(c.name, 'Uncategorized') AS category, COALESCE(-SUM(t.amount), 0) AS total
FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id AND c.budget_id = t.budget_id
WHERE t.budget_id = '<budget id>'
  AND t.deleted = FALSE
  AND t.date >= '2026-05-01' AND t.date <= '2026-05-31'
  AND t.amount < 0
  AND t.transfer_account_id IS NULL
  AND COALESCE(c.is_system, FALSE) = FALSE
GROUP BY 1
ORDER BY total DESC;

-- Transactions carrying a given tag:
SELECT t.date, t.amount FROM transactions t
JOIN tags tg ON tg.id = ANY(t.tag_ids) AND tg.budget_id = t.budget_id AND tg.deleted = FALSE
WHERE t.budget_id = '<budget id>' AND t.deleted = FALSE AND tg.name = 'Holiday';`

var allTables = map[string]tableSchema{
	"accounts": {
		Columns: []string{
			"id UUID PRIMARY KEY",
			"name TEXT NOT NULL",
			"budget_id UUID NOT NULL REFERENCES budgets(id)",
			"type TEXT NOT NULL",
			"suffix TEXT",
			"closed BOOLEAN DEFAULT false",
			"deleted BOOLEAN DEFAULT false",
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
			"status TEXT NOT NULL",
			"transfer_account_id UUID REFERENCES accounts(id)",
			"tag_ids UUID[]",
			"deleted BOOLEAN DEFAULT false",
		},
		Notes: []string{
			"amount: negative = spending, positive = income. For spend totals: COALESCE(-SUM(amount),0) WHERE amount < 0.",
			"date is TEXT YYYY-MM-DD, cast with date::date for comparisons.",
			"transfer_account_id IS NOT NULL means a transfer between the user's own accounts; exclude from spending and income.",
			"tag_ids is an array of tags.id, not a foreign key to a join table.",
			"Always filter deleted = FALSE.",
		},
	},
	"categories": {
		Columns: []string{
			"id UUID PRIMARY KEY",
			"name TEXT NOT NULL",
			"budget_id UUID NOT NULL REFERENCES budgets(id)",
			"category_group_id UUID REFERENCES category_groups(id)",
			"note TEXT",
			"hidden BOOLEAN DEFAULT false",
			"is_system BOOLEAN DEFAULT false",
			"deleted BOOLEAN DEFAULT false",
		},
		Notes: []string{
			"is_system = true marks the inflow / ready-to-assign category. Exclude it from spending breakdowns; include it when asked about money available to assign.",
			"hidden = true means the user archived the category; it may still have historical activity.",
		},
	},
	"category_groups": {
		Columns: []string{
			"id UUID PRIMARY KEY",
			"name TEXT NOT NULL",
			"budget_id UUID NOT NULL REFERENCES budgets(id)",
			"hidden BOOLEAN DEFAULT false",
			"deleted BOOLEAN DEFAULT false",
		},
	},
	"payees": {
		Columns: []string{
			"id UUID PRIMARY KEY",
			"name TEXT NOT NULL",
			"budget_id UUID NOT NULL REFERENCES budgets(id)",
			"transfer_account_id UUID REFERENCES accounts(id)",
			"deleted BOOLEAN DEFAULT false",
		},
		Notes: []string{
			"A payee with a non-null transfer_account_id represents the other side of an account transfer, not a merchant.",
		},
	},
	"tags": {
		Columns: []string{
			"id UUID PRIMARY KEY",
			"name TEXT NOT NULL",
			"budget_id UUID NOT NULL REFERENCES budgets(id)",
			"color TEXT NOT NULL",
			"deleted BOOLEAN DEFAULT false",
		},
		Notes: []string{
			"Join through the transactions.tag_ids array: tg.id = ANY(t.tag_ids).",
			"A transaction may carry several tags, so per-tag totals can sum to more than total spending.",
		},
	},
	"monthly_budgets": {
		Columns: []string{
			"id UUID PRIMARY KEY",
			"month TEXT NOT NULL",
			"budget_id UUID NOT NULL REFERENCES budgets(id)",
			"category_id UUID NOT NULL REFERENCES categories(id)",
			"budgeted NUMERIC(12, 2) NOT NULL",
			"carryover_balance NUMERIC(12, 2) NOT NULL",
		},
		Notes: []string{
			"month is YYYY-MM.",
			"A row exists only for months where an amount was assigned to the category.",
			"Prefer category_balances_by_month, which joins this to actual activity.",
		},
	},
	"category_balances_by_month": {
		Columns: []string{
			"category_id UUID",
			"category_name TEXT",
			"budget_id UUID",
			"hidden BOOLEAN",
			"is_system BOOLEAN",
			"month TEXT NOT NULL",
			"available_balance NUMERIC(12, 2)",
			"budgeted NUMERIC(12, 2)",
			"monthly_activity NUMERIC(12, 2)",
		},
		Notes: []string{
			"View over monthly_budgets and transaction activity. One row per category per month that has either a budget or activity.",
			"month is YYYY-MM (not YYYY-MM-DD). Filter: month = '2026-05'.",
			"Use available_balance directly; do not recalculate it. NULL means the category was never budgeted that month, which is not the same as zero.",
			"monthly_activity is signed like transactions.amount: negative for spending.",
			"For month comparisons use TO_DATE(month,'YYYY-MM').",
		},
	},
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
