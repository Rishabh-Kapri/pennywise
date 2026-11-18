import type { Account } from '@/features/accounts/types/account.types';

/** The parts of a statement section that say whose statement it is and for when. */
export interface StatementIdentitySource {
  accountLabel: string;
  accountNumber: string;
  kind?: 'card' | 'bank' | string;
  accountId?: string;
  periodStart?: string;
  periodEnd?: string;
  dueDate?: string;
  totalDue?: number;
}

export interface StatementIdentity {
  /** the mapped account's name, else what the statement called itself */
  name: string;
  /** last four digits of the printed number, '' when it printed none */
  ending: string;
  /** the mapped account, for its logo */
  account?: Account;
  /** account type for the logo when no account is mapped */
  logoType?: string;
  /** "13 Oct – 13 Nov 2026", '' when the statement didn't say */
  period: string;
  totalDue?: number;
  /** "3 Dec 2026", '' when not printed */
  dueBy: string;
}


/** last four digits of a printed, usually masked, account number */
export function accountEnding(number: string): string {
  const digits = number.replace(/\D/g, '');
  return digits.length >= 4 ? digits.slice(-4) : '';
}

const parseDay = (d?: string) => {
  if (!d) return null;
  const date = new Date(`${d}T00:00:00`);
  return Number.isNaN(date.getTime()) ? null : date;
};

/** "3 Dec 2026" from the API's YYYY-MM-DD; '' when missing or unreadable */
export function longDay(date?: string): string {
  const d = parseDay(date);
  return d ? d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' }) : '';
}

/** "1 Oct – 31 Oct 2026"; '' when the statement didn't print both ends */
export function formatPeriod(start?: string, end?: string): string {
  const from = parseDay(start);
  const to = parseDay(end);
  if (!from || !to) return '';
  const dayMonth = { day: 'numeric', month: 'short' } as const;
  const sameYear = from.getFullYear() === to.getFullYear();
  const left = from.toLocaleDateString(undefined, sameYear ? dayMonth : { ...dayMonth, year: 'numeric' });
  const right = to.toLocaleDateString(undefined, { ...dayMonth, year: 'numeric' });
  return `${left} – ${right}`;
}

export function statementIdentity(section: StatementIdentitySource, accounts: Account[]): StatementIdentity {
  const account = section.accountId ? accounts.find((a) => a.id === section.accountId) : undefined;
  const ending = accountEnding(section.accountNumber);
  return {
    name:
      account?.name ||
      section.accountLabel ||
      (section.kind === 'card' ? 'Credit card statement' : section.kind === 'bank' ? 'Bank statement' : 'Statement'),
    ending,
    account,
    logoType:
      account?.type ?? (section.kind === 'card' ? 'creditCard' : section.kind === 'bank' ? 'checking' : undefined),
    period: formatPeriod(section.periodStart, section.periodEnd),
    totalDue: section.totalDue,
    dueBy: longDay(section.dueDate),
  };
}
