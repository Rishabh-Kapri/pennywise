package db

import (
	"context"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReportRepository interface {
	BaseRepositoryInterface
	GetSpendingRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, categoryIds, tagIds []uuid.UUID) ([]model.SpendingRow, error)
	GetIncomeRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, tagIds []uuid.UUID) ([]model.IncomeRow, error)
	GetExpenseRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, categoryIds, tagIds []uuid.UUID) ([]model.ExpenseRow, error)
	GetNetWorthByMonth(ctx context.Context, budgetId uuid.UUID, startMonthDate, endMonthDate, startDate, endDateExcl string, accountIds []uuid.UUID) ([]model.NetWorthPoint, error)
	GetCashFlowIncomeRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, tagIds []uuid.UUID) ([]model.CashFlowIncomeRow, error)
	GetCashFlowSpendingRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, categoryIds, tagIds []uuid.UUID) ([]model.ExpenseRow, error)
	GetTransferFlowRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, tagIds []uuid.UUID) ([]model.TransferFlowRow, error)
	GetLoanBalanceInputs(ctx context.Context, budgetId uuid.UUID, endDateExcl string, accountIds []uuid.UUID) ([]model.LoanBalanceInput, error)
}

// budgetAccountTypes are the on-budget account types; everything else
// (asset, liability and the loan types) is tracked off-budget.
const budgetAccountTypes = `('checking', 'savings', 'creditCard')`

// netWorthDebtTypes are the account types whose transactions sum to their
// balance. Loan accounts are left out here: their principal lives in
// loan_metadata and payments include interest, so the report amortizes them
// separately (GetLoanBalanceInputs).
const netWorthDebtTypes = `('creditCard', 'liability')`

type reportRepo struct {
	BaseRepository
}

func NewReportRepository(pool *pgxpool.Pool) ReportRepository {
	return &reportRepo{BaseRepository: NewBaseRepository(pool)}
}

// spendingBaseWhere excludes system/hidden categories, the budget's inflow
// category and credit-card payments group, matching GetAllSimplified and the
// legacy reports behavior. Dates are TEXT 'YYYY-MM-DD'; zero-padded ISO
// strings compare correctly lexicographically so the (budget_id, date) index
// stays usable.
const spendingBaseWhere = `
	t.budget_id = $1 AND t.deleted = FALSE
	AND t.date >= $2 AND t.date < $3
	AND c.is_system = FALSE AND c.hidden = FALSE
	AND c.id != (b.metadata ->> 'inflowCategoryId')::uuid
	AND c.category_group_id != (b.metadata ->> 'ccGroupId')::uuid
`

func (r *reportRepo) GetSpendingRows(
	ctx context.Context,
	budgetId uuid.UUID,
	startDate, endDateExcl string,
	accountIds, categoryIds, tagIds []uuid.UUID,
) ([]model.SpendingRow, error) {
	query := `
		SELECT c.category_group_id, cg.name, c.id, c.name, SUM(t.amount) AS net
		FROM transactions t
		JOIN categories c ON c.id = t.category_id AND c.deleted = FALSE
		JOIN category_groups cg ON cg.id = c.category_group_id AND cg.deleted = FALSE
		JOIN budgets b ON b.id = t.budget_id
		WHERE ` + spendingBaseWhere
	args := []any{budgetId, startDate, endDateExcl}
	query, args = appendUUIDFilter(query, args, "t.account_id", accountIds)
	query, args = appendUUIDFilter(query, args, "t.category_id", categoryIds)
	query, args = appendUUIDOverlapFilter(query, args, "t.tag_ids", tagIds)
	query += ` GROUP BY c.category_group_id, cg.name, c.id, c.name`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.SpendingRow
	for rows.Next() {
		var row model.SpendingRow
		if err := rows.Scan(&row.GroupID, &row.GroupName, &row.CategoryID, &row.CategoryName, &row.Net); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *reportRepo) GetIncomeRows(
	ctx context.Context,
	budgetId uuid.UUID,
	startDate, endDateExcl string,
	accountIds, tagIds []uuid.UUID,
) ([]model.IncomeRow, error) {
	query := `
		SELECT
			TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
			COALESCE(t.payee_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(p.name, ''),
			SUM(t.amount)
		FROM transactions t
		JOIN budgets b ON b.id = t.budget_id
		LEFT JOIN payees p ON p.id = t.payee_id
		WHERE t.budget_id = $1 AND t.deleted = FALSE
			AND t.date >= $2 AND t.date < $3
			AND t.category_id = (b.metadata ->> 'inflowCategoryId')::uuid`
	args := []any{budgetId, startDate, endDateExcl}
	query, args = appendUUIDFilter(query, args, "t.account_id", accountIds)
	query, args = appendUUIDOverlapFilter(query, args, "t.tag_ids", tagIds)
	query += `
		GROUP BY month, t.payee_id, p.name
		ORDER BY month`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.IncomeRow
	for rows.Next() {
		var row model.IncomeRow
		if err := rows.Scan(&row.Month, &row.PayeeID, &row.PayeeName, &row.Amount); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *reportRepo) GetExpenseRows(
	ctx context.Context,
	budgetId uuid.UUID,
	startDate, endDateExcl string,
	accountIds, categoryIds, tagIds []uuid.UUID,
) ([]model.ExpenseRow, error) {
	query := `
		SELECT
			TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
			c.category_group_id, cg.name, c.id, c.name, SUM(t.amount) AS net
		FROM transactions t
		JOIN categories c ON c.id = t.category_id AND c.deleted = FALSE
		JOIN category_groups cg ON cg.id = c.category_group_id AND cg.deleted = FALSE
		JOIN budgets b ON b.id = t.budget_id
		WHERE ` + spendingBaseWhere
	args := []any{budgetId, startDate, endDateExcl}
	query, args = appendUUIDFilter(query, args, "t.account_id", accountIds)
	query, args = appendUUIDFilter(query, args, "t.category_id", categoryIds)
	query, args = appendUUIDOverlapFilter(query, args, "t.tag_ids", tagIds)
	query += `
		GROUP BY month, c.category_group_id, cg.name, c.id, c.name
		ORDER BY month`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.ExpenseRow
	for rows.Next() {
		var row model.ExpenseRow
		if err := rows.Scan(&row.Month, &row.GroupID, &row.GroupName, &row.CategoryID, &row.CategoryName, &row.Net); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetNetWorthByMonth returns one point per month in the range (gap months
// included via generate_series) with cumulative asset/liability balances.
// The opening CTE folds in all transactions before the window so a range
// starting mid-history is still correct. Closed accounts are included
// (their history is real); deleted accounts/transactions are excluded.
//
// startMonthDate/endMonthDate are 'YYYY-MM-01' of the first/last month,
// startDate/endDateExcl are the TEXT date bounds for the window.
func (r *reportRepo) GetNetWorthByMonth(
	ctx context.Context,
	budgetId uuid.UUID,
	startMonthDate, endMonthDate, startDate, endDateExcl string,
	accountIds []uuid.UUID,
) ([]model.NetWorthPoint, error) {
	// the optional account filter applies to both the windowed deltas and
	// the opening balance, so it is spliced into both CTEs
	accountClause := ""
	args := []any{budgetId, startMonthDate, endMonthDate, startDate, endDateExcl}
	if len(accountIds) > 0 {
		args = append(args, accountIds)
		accountClause = fmt.Sprintf(" AND t.account_id = ANY($%d)", len(args))
	}

	query := `
		WITH months AS (
			SELECT generate_series($2::date, $3::date, interval '1 month') AS month
		),
		monthly AS (
			SELECT
				date_trunc('month', t.date::date) AS month,
				SUM(CASE WHEN a.type IN ('checking', 'savings', 'asset') THEN t.amount ELSE 0 END) AS asset_delta,
				SUM(CASE WHEN a.type = 'asset' THEN t.amount ELSE 0 END) AS investment_delta,
				SUM(CASE WHEN a.type IN ` + netWorthDebtTypes + ` THEN t.amount ELSE 0 END) AS liability_delta
			FROM transactions t
			JOIN accounts a ON a.id = t.account_id AND a.deleted = FALSE
			WHERE t.budget_id = $1 AND t.deleted = FALSE
				AND t.date >= $4 AND t.date < $5` + accountClause + `
			GROUP BY 1
		),
		opening AS (
			SELECT
				COALESCE(SUM(CASE WHEN a.type IN ('checking', 'savings', 'asset') THEN t.amount END), 0) AS assets,
				COALESCE(SUM(CASE WHEN a.type = 'asset' THEN t.amount END), 0) AS investments,
				COALESCE(SUM(CASE WHEN a.type IN ` + netWorthDebtTypes + ` THEN t.amount END), 0) AS liabilities
			FROM transactions t
			JOIN accounts a ON a.id = t.account_id AND a.deleted = FALSE
			WHERE t.budget_id = $1 AND t.deleted = FALSE AND t.date < $4` + accountClause + `
		)
		SELECT
			TO_CHAR(m.month, 'YYYY-MM') AS month,
			o.assets + SUM(COALESCE(mo.asset_delta, 0)) OVER (ORDER BY m.month) AS assets,
			o.investments + SUM(COALESCE(mo.investment_delta, 0)) OVER (ORDER BY m.month) AS investments,
			o.liabilities + SUM(COALESCE(mo.liability_delta, 0)) OVER (ORDER BY m.month) AS liabilities
		FROM months m
		LEFT JOIN monthly mo ON mo.month = m.month
		CROSS JOIN opening o
		ORDER BY m.month`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.NetWorthPoint
	for rows.Next() {
		var p model.NetWorthPoint
		if err := rows.Scan(&p.Month, &p.Assets, &p.Investments, &p.Liabilities); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetCashFlowIncomeRows returns Ready to Assign inflows on on-budget
// accounts, excluding transfers (a fund sale is not earned income). Opening
// balances are flagged rather than dropped so the report can show them apart.
func (r *reportRepo) GetCashFlowIncomeRows(
	ctx context.Context,
	budgetId uuid.UUID,
	startDate, endDateExcl string,
	accountIds, tagIds []uuid.UUID,
) ([]model.CashFlowIncomeRow, error) {
	query := `
		SELECT
			TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
			COALESCE(t.payee_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(p.name, ''),
			COALESCE(t.payee_id = (b.metadata ->> 'startingBalPayeeId')::uuid, FALSE) AS opening,
			SUM(t.amount)
		FROM transactions t
		JOIN budgets b ON b.id = t.budget_id
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN payees p ON p.id = t.payee_id
		WHERE t.budget_id = $1 AND t.deleted = FALSE
			AND t.date >= $2 AND t.date < $3
			AND a.type IN ` + budgetAccountTypes + `
			AND t.transfer_account_id IS NULL
			AND t.category_id = (b.metadata ->> 'inflowCategoryId')::uuid`
	args := []any{budgetId, startDate, endDateExcl}
	query, args = appendUUIDFilter(query, args, "t.account_id", accountIds)
	query, args = appendUUIDOverlapFilter(query, args, "t.tag_ids", tagIds)
	query += `
		GROUP BY month, t.payee_id, p.name, opening
		ORDER BY month`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.CashFlowIncomeRow
	for rows.Next() {
		var row model.CashFlowIncomeRow
		if err := rows.Scan(&row.Month, &row.PayeeID, &row.PayeeName, &row.Opening, &row.Amount); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetCashFlowSpendingRows returns net activity per category and month on
// on-budget accounts, excluding transfers (money moved to an investment is
// not spent). Hidden categories stay in: their history is real spending.
// Uncategorized activity comes back under the nil group/category id.
func (r *reportRepo) GetCashFlowSpendingRows(
	ctx context.Context,
	budgetId uuid.UUID,
	startDate, endDateExcl string,
	accountIds, categoryIds, tagIds []uuid.UUID,
) ([]model.ExpenseRow, error) {
	query := `
		SELECT
			TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
			COALESCE(c.category_group_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(cg.name, 'Uncategorized'),
			COALESCE(c.id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(c.name, 'Uncategorized'),
			SUM(t.amount) AS net
		FROM transactions t
		JOIN budgets b ON b.id = t.budget_id
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN categories c ON c.id = t.category_id
		LEFT JOIN category_groups cg ON cg.id = c.category_group_id
		WHERE t.budget_id = $1 AND t.deleted = FALSE
			AND t.date >= $2 AND t.date < $3
			AND a.type IN ` + budgetAccountTypes + `
			AND t.transfer_account_id IS NULL
			AND (c.id IS NULL OR (
				c.is_system = FALSE
				AND c.id != (b.metadata ->> 'inflowCategoryId')::uuid
				AND c.category_group_id != (b.metadata ->> 'ccGroupId')::uuid
			))`
	args := []any{budgetId, startDate, endDateExcl}
	query, args = appendUUIDFilter(query, args, "t.account_id", accountIds)
	query, args = appendUUIDFilter(query, args, "t.category_id", categoryIds)
	query, args = appendUUIDOverlapFilter(query, args, "t.tag_ids", tagIds)
	query += `
		GROUP BY 1, 2, 3, 4, 5
		ORDER BY month`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.ExpenseRow
	for rows.Next() {
		var row model.ExpenseRow
		if err := rows.Scan(&row.Month, &row.GroupID, &row.GroupName, &row.CategoryID, &row.CategoryName, &row.Net); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetTransferFlowRows returns, per month and off-budget account, the money
// moved between it and on-budget accounts, read from the on-budget leg. The
// leg's category is ignored for In/Out; inflows that carry a regular
// category (not Ready to Assign) are counted separately so the report can
// say how many were filed that way.
func (r *reportRepo) GetTransferFlowRows(
	ctx context.Context,
	budgetId uuid.UUID,
	startDate, endDateExcl string,
	accountIds, tagIds []uuid.UUID,
) ([]model.TransferFlowRow, error) {
	query := `
		SELECT
			TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month,
			ta.id, ta.name, ta.type, ta.closed,
			COALESCE(SUM(t.amount) FILTER (WHERE t.amount > 0), 0) AS amount_in,
			COALESCE(-SUM(t.amount) FILTER (WHERE t.amount < 0), 0) AS amount_out,
			COUNT(*) FILTER (WHERE t.amount > 0 AND t.category_id IS NOT NULL
				AND t.category_id != (b.metadata ->> 'inflowCategoryId')::uuid) AS recat_count,
			COALESCE(SUM(t.amount) FILTER (WHERE t.amount > 0 AND t.category_id IS NOT NULL
				AND t.category_id != (b.metadata ->> 'inflowCategoryId')::uuid), 0) AS recat_amount
		FROM transactions t
		JOIN budgets b ON b.id = t.budget_id
		JOIN accounts a ON a.id = t.account_id
		JOIN accounts ta ON ta.id = t.transfer_account_id
		WHERE t.budget_id = $1 AND t.deleted = FALSE
			AND t.date >= $2 AND t.date < $3
			AND a.type IN ` + budgetAccountTypes + `
			AND ta.type NOT IN ` + budgetAccountTypes
	args := []any{budgetId, startDate, endDateExcl}
	query, args = appendUUIDFilter(query, args, "t.account_id", accountIds)
	query, args = appendUUIDOverlapFilter(query, args, "t.tag_ids", tagIds)
	query += `
		GROUP BY 1, ta.id, ta.name, ta.type, ta.closed
		ORDER BY month`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.TransferFlowRow
	for rows.Next() {
		var row model.TransferFlowRow
		if err := rows.Scan(&row.Month, &row.AccountID, &row.AccountName, &row.AccountType, &row.Closed,
			&row.In, &row.Out, &row.RecategorizedCount, &row.RecategorizedAmount); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetLoanBalanceInputs returns every loan account that has loan metadata,
// with the net amount booked to it per month before endDateExcl. Accounts
// are filtered by accountIds when given, like the rest of net worth.
func (r *reportRepo) GetLoanBalanceInputs(
	ctx context.Context,
	budgetId uuid.UUID,
	endDateExcl string,
	accountIds []uuid.UUID,
) ([]model.LoanBalanceInput, error) {
	query := `
		SELECT
			a.id, lm.loan_start_date, lm.original_balance::float8, lm.interest_rate::float8, lm.monthly_payment::float8,
			p.month, COALESCE(p.amount, 0)
		FROM accounts a
		JOIN loan_metadata lm ON lm.account_id = a.id AND lm.deleted = FALSE
		LEFT JOIN LATERAL (
			SELECT TO_CHAR(date_trunc('month', t.date::date), 'YYYY-MM') AS month, SUM(t.amount)::float8 AS amount
			FROM transactions t
			WHERE t.account_id = a.id AND t.deleted = FALSE AND t.date < $2
			GROUP BY 1
		) p ON TRUE
		WHERE a.budget_id = $1 AND a.deleted = FALSE`
	args := []any{budgetId, endDateExcl}
	query, args = appendUUIDFilter(query, args, "a.id", accountIds)
	query += ` ORDER BY a.id, p.month`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.LoanBalanceInput
	for rows.Next() {
		var (
			loan   model.LoanBalanceInput
			month  *string
			amount float64
		)
		if err := rows.Scan(&loan.AccountID, &loan.StartDate, &loan.OriginalBalance, &loan.InterestRate,
			&loan.MonthlyPayment, &month, &amount); err != nil {
			return nil, err
		}
		if n := len(result); n == 0 || result[n-1].AccountID != loan.AccountID {
			loan.Payments = map[string]float64{}
			result = append(result, loan)
		}
		if month != nil {
			result[len(result)-1].Payments[*month] = amount
		}
	}
	return result, rows.Err()
}

// appendUUIDFilter appends an "AND col = ANY($n)" clause when ids is
// non-empty; pgx binds []uuid.UUID natively.
func appendUUIDFilter(query string, args []any, column string, ids []uuid.UUID) (string, []any) {
	if len(ids) == 0 {
		return query, args
	}
	args = append(args, ids)
	return query + fmt.Sprintf(" AND %s = ANY($%d)", column, len(args)), args
}

// appendUUIDOverlapFilter appends an "AND col && $n" clause for uuid[]
// columns, matching rows carrying any of the given ids.
func appendUUIDOverlapFilter(query string, args []any, column string, ids []uuid.UUID) (string, []any) {
	if len(ids) == 0 {
		return query, args
	}
	args = append(args, ids)
	return query + fmt.Sprintf(" AND %s && $%d", column, len(args)), args
}
