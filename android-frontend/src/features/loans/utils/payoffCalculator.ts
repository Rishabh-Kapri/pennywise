import type { PayoffInput } from '../types';

// Same projection as react-frontend `loans/utils/payoffCalculator.ts`.
const MAX_MONTHS = 720;

export interface PayoffProjection {
  months: number;
  totalInterest: number;
  /** interest in the first projected payment */
  nextInterest: number;
  /** false when the payment doesn't even cover the interest */
  paysOff: boolean;
}

/** Months to clear `currentBalance` and the interest paid on the way. */
export function projectPayoff(input: PayoffInput): PayoffProjection {
  const monthlyRate = input.interestRate / 100 / 12;
  const payment = input.monthlyPayment + (input.extraMonthlyPayment ?? 0);
  let balance = input.currentBalance;
  let totalInterest = 0;
  let nextInterest = 0;
  let months = 0;

  while (balance > 0.005 && months < MAX_MONTHS) {
    months += 1;
    const interest = balance * monthlyRate;
    if (months === 1) nextInterest = interest;
    const oneTime = months === 1 ? input.oneTimeExtraPayment ?? 0 : 0;
    const principal = Math.min(payment - interest + oneTime, balance);
    if (principal <= 0) return { months: Infinity, totalInterest: Infinity, nextInterest, paysOff: false };
    balance -= principal;
    totalInterest += interest;
  }

  return { months, totalInterest, nextInterest, paysOff: balance <= 0.005 };
}

/** "2 years 3 months" */
export function formatPayoffDuration(months: number): string {
  if (!Number.isFinite(months)) return 'never';
  const years = Math.floor(months / 12);
  const rest = months % 12;
  const y = `${years} year${years === 1 ? '' : 's'}`;
  const m = `${rest} month${rest === 1 ? '' : 's'}`;
  if (!years) return m;
  return rest ? `${y} ${m}` : y;
}
