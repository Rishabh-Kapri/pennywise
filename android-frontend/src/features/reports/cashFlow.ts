import type { AccountFlow, CashFlowReport, GroupTreatment } from './types';

/** The nil uuid the API uses for uncategorized spending. */
const NIL_ID = '00000000-0000-0000-0000-000000000000';

/**
 * Guess a group's treatment from its name so the common setups ("One time
 * expenses", "Investments", "Savings goals") read right without any clicks.
 * Users override it per group.
 */
export function defaultTreatment(groupId: string, name: string): GroupTreatment {
  if (groupId === NIL_ID) return 'spending';
  if (/one[\s-]?(time|off)|once/i.test(name)) return 'oneOff';
  if (/invest|saving|retire/i.test(name)) return 'saving';
  return 'spending';
}

export type Monthly = Record<string, number>;

export interface LineSummary {
  id: string;
  name: string;
  total: number;
  monthly: Monthly;
}

export interface GroupSummary extends LineSummary {
  treatment: GroupTreatment;
  /** True when the treatment is the name-based guess, not a user choice. */
  isDefault: boolean;
  categories: LineSummary[];
}

export interface IncomeSourceSummary extends LineSummary {
  /** Months in the window with any income from this source. */
  activeMonths: number;
}

export interface AccountFlowSummary {
  id: string;
  name: string;
  type: string;
  closed: boolean;
  in: number;
  out: number;
  inMonthly: Monthly;
  outMonthly: Monthly;
  recategorizedCount: number;
  recategorizedAmount: number;
}

export interface CashFlowSummary {
  /** Months from the first one with any activity to the end of the range. */
  months: string[];
  income: number;
  incomeMonthly: Monthly;
  incomeSources: IncomeSourceSummary[];
  /** Everyday spending: groups treated as spending. */
  spending: number;
  spendingMonthly: Monthly;
  oneOff: number;
  oneOffMonthly: Monthly;
  /** Spending in groups treated as saving; part of `invested`. */
  savedInGroups: number;
  /** Moved into asset accounts, plus saving groups. */
  invested: number;
  investedMonthly: Monthly;
  /** Moved out of asset accounts into the budget (fund sales etc). */
  withdrawn: number;
  withdrawnMonthly: Monthly;
  debtPaid: number;
  debtPaidMonthly: Monthly;
  borrowed: number;
  borrowedMonthly: Monthly;
  opening: number;
  groups: GroupSummary[];
  assetAccounts: AccountFlowSummary[];
  debtAccounts: AccountFlowSummary[];
  recategorizedCount: number;
  recategorizedAmount: number;
  /**
   * Money in minus money out over the window: positive means on-budget
   * accounts grew from these flows, negative means balances paid the gap.
   */
  cashChange: number;
}

export function sumMonthly(monthly: Monthly): number {
  return Object.values(monthly).reduce((sum, value) => sum + value, 0);
}

function addInto(target: Monthly, source: Monthly) {
  for (const [month, value] of Object.entries(source)) {
    target[month] = (target[month] ?? 0) + value;
  }
}

function zeroMonthly(months: string[]): Monthly {
  return Object.fromEntries(months.map((month) => [month, 0]));
}

function summarizeAccount(flow: AccountFlow): AccountFlowSummary {
  return {
    id: flow.accountId,
    name: flow.name,
    type: flow.type,
    closed: flow.closed,
    in: sumMonthly(flow.in),
    out: sumMonthly(flow.out),
    inMonthly: flow.in,
    outMonthly: flow.out,
    recategorizedCount: flow.recategorizedCount,
    recategorizedAmount: flow.recategorizedAmount,
  };
}

/** First month with anything recorded, so "All time" averages aren't diluted. */
function activeMonths(report: CashFlowReport): string[] {
  const hasActivity = (month: string) =>
    (report.income.totals[month] ?? 0) !== 0 ||
    (report.spending.totals[month] ?? 0) !== 0 ||
    report.transfers.some((flow) => (flow.in[month] ?? 0) !== 0 || (flow.out[month] ?? 0) !== 0);
  const first = report.months.findIndex(hasActivity);
  return first === -1 ? report.months : report.months.slice(first);
}

export function summarizeCashFlow(report: CashFlowReport, overrides: Record<string, GroupTreatment>): CashFlowSummary {
  const months = activeMonths(report);

  const incomeSources: IncomeSourceSummary[] = report.income.payees
    .map((payee) => ({
      id: payee.payeeId,
      name: payee.name || 'No payee',
      total: sumMonthly(payee.amounts),
      monthly: payee.amounts,
      activeMonths: months.filter((month) => (payee.amounts[month] ?? 0) > 0).length,
    }))
    .filter((source) => Math.round(source.total) !== 0)
    .sort((a, b) => b.total - a.total);

  const spendingMonthly = zeroMonthly(months);
  const oneOffMonthly = zeroMonthly(months);
  const savedMonthly = zeroMonthly(months);
  const groups: GroupSummary[] = report.spending.groups
    .map((group) => {
      const guess = defaultTreatment(group.categoryGroupId, group.name);
      const treatment = overrides[group.categoryGroupId] ?? guess;
      return {
        id: group.categoryGroupId,
        name: group.name,
        total: sumMonthly(group.totals),
        monthly: group.totals,
        treatment,
        isDefault: overrides[group.categoryGroupId] === undefined,
        categories: group.categories
          .map((category) => ({
            id: category.categoryId,
            name: category.name,
            total: sumMonthly(category.amounts),
            monthly: category.amounts,
          }))
          .filter((category) => Math.round(category.total) !== 0)
          .sort((a, b) => b.total - a.total),
      };
    })
    .filter((group) => Math.round(group.total) !== 0)
    .sort((a, b) => b.total - a.total);

  for (const group of groups) {
    const target =
      group.treatment === 'oneOff' ? oneOffMonthly : group.treatment === 'saving' ? savedMonthly : spendingMonthly;
    addInto(target, group.monthly);
  }

  const assetAccounts = report.transfers.filter((flow) => flow.kind === 'asset').map(summarizeAccount);
  const debtAccounts = report.transfers.filter((flow) => flow.kind === 'debt').map(summarizeAccount);

  const investedMonthly = { ...savedMonthly };
  const withdrawnMonthly = zeroMonthly(months);
  for (const account of assetAccounts) {
    addInto(investedMonthly, account.outMonthly);
    addInto(withdrawnMonthly, account.inMonthly);
  }
  const debtPaidMonthly = zeroMonthly(months);
  const borrowedMonthly = zeroMonthly(months);
  for (const account of debtAccounts) {
    addInto(debtPaidMonthly, account.outMonthly);
    addInto(borrowedMonthly, account.inMonthly);
  }

  const income = sumMonthly(report.income.totals);
  const spending = sumMonthly(spendingMonthly);
  const oneOff = sumMonthly(oneOffMonthly);
  const invested = sumMonthly(investedMonthly);
  const withdrawn = sumMonthly(withdrawnMonthly);
  const debtPaid = sumMonthly(debtPaidMonthly);
  const borrowed = sumMonthly(borrowedMonthly);
  const recategorized = report.transfers.filter((flow) => flow.recategorizedCount > 0);

  return {
    months,
    income,
    incomeMonthly: report.income.totals,
    incomeSources,
    spending,
    spendingMonthly,
    oneOff,
    oneOffMonthly,
    savedInGroups: sumMonthly(savedMonthly),
    invested,
    investedMonthly,
    withdrawn,
    withdrawnMonthly,
    debtPaid,
    debtPaidMonthly,
    borrowed,
    borrowedMonthly,
    opening: sumMonthly(report.openingBalances),
    groups,
    assetAccounts,
    debtAccounts,
    recategorizedCount: recategorized.reduce((sum, flow) => sum + flow.recategorizedCount, 0),
    recategorizedAmount: recategorized.reduce((sum, flow) => sum + flow.recategorizedAmount, 0),
    cashChange: income + withdrawn + borrowed - (spending + oneOff + invested + debtPaid),
  };
}

/** value / income as a percentage, or null when there was no income. */
export function shareOfIncome(value: number, income: number): number | null {
  return income > 0 ? (value / income) * 100 : null;
}

export function formatPercent(value: number | null): string {
  if (value === null) return '—';
  if (Math.abs(value) >= 10) return `${Math.round(value)}%`;
  return `${value.toFixed(1).replace(/\.0$/, '')}%`;
}

export function monthLabel(monthKey: string, withYear = true): string {
  const [year, month] = monthKey.split('-');
  const date = new Date(parseInt(year, 10), parseInt(month, 10) - 1, 1);
  const name = date.toLocaleString('en-us', { month: 'short' });
  return withYear ? `${name} ’${year.slice(2)}` : name;
}
