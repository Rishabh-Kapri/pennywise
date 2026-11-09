package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type ReportRepository interface {
	GetSpendingRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, categoryIds, tagIds []uuid.UUID) ([]model.SpendingRow, error)
	GetIncomeRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, tagIds []uuid.UUID) ([]model.IncomeRow, error)
	GetExpenseRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, categoryIds, tagIds []uuid.UUID) ([]model.ExpenseRow, error)
	GetNetWorthByMonth(ctx context.Context, budgetId uuid.UUID, startMonthDate, endMonthDate, startDate, endDateExcl string, accountIds []uuid.UUID) ([]model.NetWorthPoint, error)
	GetCashFlowIncomeRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, tagIds []uuid.UUID) ([]model.CashFlowIncomeRow, error)
	GetCashFlowSpendingRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, categoryIds, tagIds []uuid.UUID) ([]model.ExpenseRow, error)
	GetTransferFlowRows(ctx context.Context, budgetId uuid.UUID, startDate, endDateExcl string, accountIds, tagIds []uuid.UUID) ([]model.TransferFlowRow, error)
	GetLoanBalanceInputs(ctx context.Context, budgetId uuid.UUID, endDateExcl string, accountIds []uuid.UUID) ([]model.LoanBalanceInput, error)
}
