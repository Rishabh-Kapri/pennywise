import type { Account } from '../accounts/types';

/** Whose statement it is, for when, and what's due (mirrors the web's statementIdentity.ts). */
export interface StatementIdentitySource {
  accountLabel: string;
  accountNumber: string;
  kind?: string;
  accountId?: string;
  periodStart?: string;
  periodEnd?: string;
  dueDate?: string;
  totalDue?: number;
}

export interface StatementIdentity {
  name: string;
  /** last four digits of the printed number, '' when none */
  ending: string;
  isCard: boolean;
  /** "13 Oct – 13 Nov 2026", '' when not printed */
  period: string;
  totalDue?: number;
  /** "3 Dec 2026", '' when not printed */
  dueBy: string;
}

/** Paise kept: a reconciliation that rounds hides the 0.76 it is about. */
export const money = (value: number) =>
  new Intl.NumberFormat('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);

export const signedMoney = (value: number) => `${value < 0 ? '−' : '+'}${money(Math.abs(value))}`;

export function accountEnding(number: string): string {
  const digits = number.replace(/\D/g, '');
  return digits.length >= 4 ? digits.slice(-4) : '';
}

const parseDay = (d?: string) => {
  if (!d) return null;
  const date = new Date(`${d.slice(0, 10)}T00:00:00`);
  return Number.isNaN(date.getTime()) ? null : date;
};

/** "03 Nov" */
export function shortDay(date: string): string {
  const d = parseDay(date);
  return d ? d.toLocaleDateString('en-IN', { day: '2-digit', month: 'short' }) : date;
}

export function longDay(date?: string): string {
  const d = parseDay(date);
  return d ? d.toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' }) : '';
}

export function formatPeriod(start?: string, end?: string): string {
  const from = parseDay(start);
  const to = parseDay(end);
  if (!from || !to) return '';
  const dayMonth = { day: 'numeric', month: 'short' } as const;
  const sameYear = from.getFullYear() === to.getFullYear();
  const left = from.toLocaleDateString('en-IN', sameYear ? dayMonth : { ...dayMonth, year: 'numeric' });
  const right = to.toLocaleDateString('en-IN', { ...dayMonth, year: 'numeric' });
  return `${left} – ${right}`;
}

export function statementIdentity(section: StatementIdentitySource, accounts: Account[]): StatementIdentity {
  const account = section.accountId ? accounts.find((a) => a.id === section.accountId) : undefined;
  const isCard = account ? account.type === 'creditCard' : section.kind === 'card';
  return {
    name:
      account?.name ||
      section.accountLabel ||
      (isCard ? 'Credit card statement' : section.kind === 'bank' ? 'Bank statement' : 'Statement'),
    ending: accountEnding(section.accountNumber),
    isCard,
    period: formatPeriod(section.periodStart, section.periodEnd),
    totalDue: section.totalDue,
    dueBy: longDay(section.dueDate)
  };
}
