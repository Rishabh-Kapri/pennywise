import type { LoanMetadata } from '../types';

export function getLoanProjection(loan: LoanMetadata, paidSoFar: number) {
  const currentBalance = Math.max(loan.originalBalance - Math.abs(paidSoFar), 0);
  const monthlyRate = loan.interestRate / 100 / 12;
  let balance = currentBalance;
  let totalInterest = 0;
  let months = 0;

  while (balance > 0 && months < 600) {
    const interest = balance * monthlyRate;
    const principal = Math.min(Math.max(loan.monthlyPayment - interest, 0), balance);
    if (principal <= 0) break;
    balance -= principal;
    totalInterest += interest;
    months += 1;
  }

  return {
    currentBalance,
    totalInterest,
    months,
    percentPaid: loan.originalBalance > 0 ? Math.min(Math.round((Math.abs(paidSoFar) / loan.originalBalance) * 100), 100) : 0
  };
}
