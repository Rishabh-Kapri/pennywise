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
}

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
				SUM(CASE WHEN a.type IN ('creditCard', 'liability') THEN t.amount ELSE 0 END) AS liability_delta
			FROM transactions t
			JOIN accounts a ON a.id = t.account_id AND a.deleted = FALSE
			WHERE t.budget_id = $1 AND t.deleted = FALSE
				AND t.date >= $4 AND t.date < $5` + accountClause + `
			GROUP BY 1
		),
		opening AS (
			SELECT
				COALESCE(SUM(CASE WHEN a.type IN ('checking', 'savings', 'asset') THEN t.amount END), 0) AS assets,
				COALESCE(SUM(CASE WHEN a.type IN ('creditCard', 'liability') THEN t.amount END), 0) AS liabilities
			FROM transactions t
			JOIN accounts a ON a.id = t.account_id AND a.deleted = FALSE
			WHERE t.budget_id = $1 AND t.deleted = FALSE AND t.date < $4` + accountClause + `
		)
		SELECT
			TO_CHAR(m.month, 'YYYY-MM') AS month,
			o.assets + SUM(COALESCE(mo.asset_delta, 0)) OVER (ORDER BY m.month) AS assets,
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
		if err := rows.Scan(&p.Month, &p.Assets, &p.Liabilities); err != nil {
			return nil, err
		}
		p.NetWorth = p.Assets + p.Liabilities
		result = append(result, p)
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
