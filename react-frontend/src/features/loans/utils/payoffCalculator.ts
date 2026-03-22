import type {
  LoanPayoffProjection,
  PayoffComparison,
  PayoffSimulatorInput,
} from '../types/loan.types';

const MAX_MONTHS = 360 * 2; // 60 years cap to avoid infinite loops

/**
 * Calculate monthly interest from an annual rate for a given balance
 */
export function calculateMonthlyInterest(
  balance: number,
  annualRate: number,
): number {
  const monthlyRate = annualRate / 100 / 12;
  return balance * monthlyRate;
}

/**
 * Generate a full amortization schedule
 */
export function calculateAmortizationSchedule(
  input: PayoffSimulatorInput,
): LoanPayoffProjection[] {
  const { currentBalance, interestRate, monthlyPayment } = input;
  const extraMonthly = input.extraMonthlyPayment ?? 0;
  const oneTimeExtra = input.oneTimeExtraPayment ?? 0;
  const oneTimeMonth = input.oneTimeExtraPaymentMonth ?? 1;

  const schedule: LoanPayoffProjection[] = [];
  let balance = currentBalance;
  let totalInterestPaid = 0;

  const startDate = new Date();

  for (let month = 1; month <= MAX_MONTHS && balance > 0; month++) {
    const interestPayment = calculateMonthlyInterest(balance, interestRate);
    const totalPayment = monthlyPayment + extraMonthly;
    const oneTime = month === oneTimeMonth ? oneTimeExtra : 0;

    // Principal is total payment minus interest (plus any one-time extra)
    let principalPayment = totalPayment - interestPayment + oneTime;

    // Can't pay more than the remaining balance + interest
    if (principalPayment > balance) {
      principalPayment = balance;
    }

    balance = Math.max(0, balance - principalPayment);
    totalInterestPaid += interestPayment;

    const projectionDate = new Date(startDate);
    projectionDate.setMonth(projectionDate.getMonth() + month);

    schedule.push({
      month,
      date: projectionDate.toISOString().split('T')[0],
      principalPayment: Math.round(principalPayment * 100) / 100,
      interestPayment: Math.round(interestPayment * 100) / 100,
      extraPayment: Math.round((extraMonthly + oneTime) * 100) / 100,
      remainingBalance: Math.round(balance * 100) / 100,
      totalInterestPaid: Math.round(totalInterestPaid * 100) / 100,
    });

    if (balance <= 0) break;
  }

  return schedule;
}

/**
 * Calculate projected payoff date
 */
export function calculatePayoffDate(input: PayoffSimulatorInput): Date {
  const schedule = calculateAmortizationSchedule(input);
  if (schedule.length === 0) return new Date();
  return new Date(schedule[schedule.length - 1].date);
}

/**
 * Calculate total interest over the life of the loan
 */
export function calculateTotalInterest(input: PayoffSimulatorInput): number {
  const schedule = calculateAmortizationSchedule(input);
  if (schedule.length === 0) return 0;
  return schedule[schedule.length - 1].totalInterestPaid;
}

/**
 * Compare two payoff scenarios and return savings
 */
export function compareScenarios(
  baseInput: PayoffSimulatorInput,
  targetInput: PayoffSimulatorInput,
): PayoffComparison {
  const baseSchedule = calculateAmortizationSchedule(baseInput);
  const targetSchedule = calculateAmortizationSchedule(targetInput);

  const baseMonths = baseSchedule.length;
  const targetMonths = targetSchedule.length;
  const baseTotalInterest =
    baseSchedule.length > 0
      ? baseSchedule[baseSchedule.length - 1].totalInterestPaid
      : 0;
  const targetTotalInterest =
    targetSchedule.length > 0
      ? targetSchedule[targetSchedule.length - 1].totalInterestPaid
      : 0;

  return {
    interestSaved: Math.round((baseTotalInterest - targetTotalInterest) * 100) / 100,
    monthsSaved: baseMonths - targetMonths,
    basePayoffMonths: baseMonths,
    targetPayoffMonths: targetMonths,
    baseTotalInterest: Math.round(baseTotalInterest * 100) / 100,
    targetTotalInterest: Math.round(targetTotalInterest * 100) / 100,
  };
}

/**
 * Format months into a human-readable string like "2 years 3 months"
 */
export function formatPayoffDuration(months: number): string {
  const years = Math.floor(months / 12);
  const remainingMonths = months % 12;

  if (years === 0) return `${remainingMonths} month${remainingMonths !== 1 ? 's' : ''}`;
  if (remainingMonths === 0) return `${years} year${years !== 1 ? 's' : ''}`;
  return `${years} year${years !== 1 ? 's' : ''} ${remainingMonths} month${remainingMonths !== 1 ? 's' : ''}`;
}
