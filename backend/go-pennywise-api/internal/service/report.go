package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// ErrInvalidReportRange is returned when startMonth is after endMonth;
// handlers map it to a 400 response.
var ErrInvalidReportRange = errors.New("startMonth must not be after endMonth")

const defaultReportMonths = 6

type ReportService interface {
	GetSpending(ctx context.Context, params model.ReportParams) (*model.SpendingReport, error)
	GetIncomeExpense(ctx context.Context, params model.ReportParams) (*model.IncomeExpenseReport, error)
	GetNetWorth(ctx context.Context, params model.ReportParams) (*model.NetWorthReport, error)
}

type reportService struct {
	repo repository.ReportRepository
}

func NewReportService(r repository.ReportRepository) ReportService {
	return &reportService{repo: r}
}

func (s *reportService) GetSpending(ctx context.Context, params model.ReportParams) (*model.SpendingReport, error) {
	budgetId := utils.MustBudgetID(ctx)
	startMonth, endMonth, err := normalizeMonthRange(params.StartMonth, params.EndMonth)
	if err != nil {
		return nil, err
	}
	startDate, endDateExcl := monthBounds(startMonth, endMonth)

	rows, err := s.repo.GetSpendingRows(ctx, budgetId, startDate, endDateExcl, params.AccountIDs, params.CategoryIDs)
	if err != nil {
		return nil, err
	}

	report := assembleSpendingReport(rows)
	report.StartMonth = startMonth
	report.EndMonth = endMonth
	return report, nil
}

func (s *reportService) GetIncomeExpense(ctx context.Context, params model.ReportParams) (*model.IncomeExpenseReport, error) {
	budgetId := utils.MustBudgetID(ctx)
	startMonth, endMonth, err := normalizeMonthRange(params.StartMonth, params.EndMonth)
	if err != nil {
		return nil, err
	}
	startDate, endDateExcl := monthBounds(startMonth, endMonth)

	incomeRows, err := s.repo.GetIncomeRows(ctx, budgetId, startDate, endDateExcl, params.AccountIDs)
	if err != nil {
		return nil, err
	}
	expenseRows, err := s.repo.GetExpenseRows(ctx, budgetId, startDate, endDateExcl, params.AccountIDs, params.CategoryIDs)
	if err != nil {
		return nil, err
	}

	report := assembleIncomeExpenseReport(monthRange(startMonth, endMonth), incomeRows, expenseRows)
	report.StartMonth = startMonth
	report.EndMonth = endMonth
	return report, nil
}

func (s *reportService) GetNetWorth(ctx context.Context, params model.ReportParams) (*model.NetWorthReport, error) {
	budgetId := utils.MustBudgetID(ctx)
	startMonth, endMonth, err := normalizeMonthRange(params.StartMonth, params.EndMonth)
	if err != nil {
		return nil, err
	}
	startDate, endDateExcl := monthBounds(startMonth, endMonth)

	points, err := s.repo.GetNetWorthByMonth(ctx, budgetId, startMonth+"-01", endMonth+"-01", startDate, endDateExcl)
	if err != nil {
		return nil, err
	}
	if points == nil {
		points = []model.NetWorthPoint{}
	}
	return &model.NetWorthReport{StartMonth: startMonth, EndMonth: endMonth, Months: points}, nil
}

// normalizeMonthRange fills in defaults (endMonth = current month,
// startMonth = a defaultReportMonths window ending there) and rejects
// inverted ranges. Format validation happens in the handler.
func normalizeMonthRange(startMonth, endMonth string) (string, string, error) {
	if endMonth == "" {
		endMonth = time.Now().Format("2006-01")
	}
	if startMonth == "" {
		startMonth = addMonthsToKey(endMonth, -(defaultReportMonths - 1))
	}
	if startMonth > endMonth {
		return "", "", ErrInvalidReportRange
	}
	return startMonth, endMonth, nil
}

// monthBounds converts an inclusive month range into TEXT date bounds:
// startDate = first day of startMonth, endDateExcl = first day of the month
// after endMonth. Zero-padded ISO strings compare lexicographically.
func monthBounds(startMonth, endMonth string) (string, string) {
	return startMonth + "-01", addMonthsToKey(endMonth, 1) + "-01"
}

// addMonthsToKey shifts a "YYYY-MM" key by delta months (handles year
// rollover via time.AddDate).
func addMonthsToKey(monthKey string, delta int) string {
	t, err := time.Parse("2006-01", monthKey)
	if err != nil {
		return monthKey
	}
	return t.AddDate(0, delta, 0).Format("2006-01")
}

// monthRange returns every "YYYY-MM" key from start to end inclusive.
func monthRange(startMonth, endMonth string) []string {
	months := []string{}
	for m := startMonth; m <= endMonth; m = addMonthsToKey(m, 1) {
		months = append(months, m)
	}
	return months
}

// assembleSpendingReport groups flat rows per category group. Totals are
// ABS of the summed amounts (legacy reports behavior); zero rows are
// dropped and groups/categories sorted by total desc.
func assembleSpendingReport(rows []model.SpendingRow) *model.SpendingReport {
	groupsById := map[uuid.UUID]*model.SpendingGroupReport{}
	order := []uuid.UUID{}

	for _, row := range rows {
		total := math.Abs(row.Net)
		if total == 0 {
			continue
		}
		group, ok := groupsById[row.GroupID]
		if !ok {
			group = &model.SpendingGroupReport{CategoryGroupID: row.GroupID, Name: row.GroupName}
			groupsById[row.GroupID] = group
			order = append(order, row.GroupID)
		}
		group.Categories = append(group.Categories, model.SpendingCategoryReport{
			CategoryID: row.CategoryID,
			Name:       row.CategoryName,
			Total:      total,
		})
		group.Total += total
	}

	report := &model.SpendingReport{Groups: []model.SpendingGroupReport{}}
	for _, id := range order {
		group := groupsById[id]
		sort.Slice(group.Categories, func(i, j int) bool {
			return group.Categories[i].Total > group.Categories[j].Total
		})
		report.Total += group.Total
		report.Groups = append(report.Groups, *group)
	}
	sort.Slice(report.Groups, func(i, j int) bool {
		return report.Groups[i].Total > report.Groups[j].Total
	})
	return report
}

// assembleIncomeExpenseReport builds the month-keyed matrices. Expense
// amounts are negated (spending stored as negative amounts becomes a
// positive expense; a net refund month shows as negative expense).
func assembleIncomeExpenseReport(months []string, incomeRows []model.IncomeRow, expenseRows []model.ExpenseRow) *model.IncomeExpenseReport {
	report := &model.IncomeExpenseReport{
		Months:  months,
		Income:  model.IncomeMatrix{Payees: []model.PayeeMonthlyAmounts{}, Totals: map[string]float64{}},
		Expense: model.ExpenseMatrix{Groups: []model.ExpenseGroupMonthly{}, Totals: map[string]float64{}},
		Net:     map[string]float64{},
	}
	for _, m := range months {
		report.Income.Totals[m] = 0
		report.Expense.Totals[m] = 0
	}

	payeesById := map[uuid.UUID]*model.PayeeMonthlyAmounts{}
	payeeOrder := []uuid.UUID{}
	for _, row := range incomeRows {
		payee, ok := payeesById[row.PayeeID]
		if !ok {
			payee = &model.PayeeMonthlyAmounts{PayeeID: row.PayeeID, Name: row.PayeeName, Amounts: map[string]float64{}}
			payeesById[row.PayeeID] = payee
			payeeOrder = append(payeeOrder, row.PayeeID)
		}
		payee.Amounts[row.Month] += row.Amount
		report.Income.Totals[row.Month] += row.Amount
	}
	for _, id := range payeeOrder {
		report.Income.Payees = append(report.Income.Payees, *payeesById[id])
	}

	groupsById := map[uuid.UUID]*model.ExpenseGroupMonthly{}
	groupOrder := []uuid.UUID{}
	categoriesById := map[uuid.UUID]*model.CategoryMonthlyAmounts{}
	categoryGroup := map[uuid.UUID]uuid.UUID{}
	categoryOrder := []uuid.UUID{}
	for _, row := range expenseRows {
		amount := -row.Net
		group, ok := groupsById[row.GroupID]
		if !ok {
			group = &model.ExpenseGroupMonthly{
				CategoryGroupID: row.GroupID,
				Name:            row.GroupName,
				Totals:          map[string]float64{},
				Categories:      []model.CategoryMonthlyAmounts{},
			}
			groupsById[row.GroupID] = group
			groupOrder = append(groupOrder, row.GroupID)
		}
		category, ok := categoriesById[row.CategoryID]
		if !ok {
			category = &model.CategoryMonthlyAmounts{CategoryID: row.CategoryID, Name: row.CategoryName, Amounts: map[string]float64{}}
			categoriesById[row.CategoryID] = category
			categoryGroup[row.CategoryID] = row.GroupID
			categoryOrder = append(categoryOrder, row.CategoryID)
		}
		category.Amounts[row.Month] += amount
		group.Totals[row.Month] += amount
		report.Expense.Totals[row.Month] += amount
	}
	for _, id := range categoryOrder {
		group := groupsById[categoryGroup[id]]
		group.Categories = append(group.Categories, *categoriesById[id])
	}
	for _, id := range groupOrder {
		report.Expense.Groups = append(report.Expense.Groups, *groupsById[id])
	}

	for _, m := range months {
		report.Net[m] = report.Income.Totals[m] - report.Expense.Totals[m]
	}
	return report
}
