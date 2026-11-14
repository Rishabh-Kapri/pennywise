import type { TransactionSource } from '../types/transaction.types';

// Banks state their own clock, and cipher reads it as IST. Showing it in the
// browser's zone would move the time off the date the transaction carries.
const BANK_TIME_ZONE = 'Asia/Kolkata';

/** "2:35 PM" for a time the bank gave, or '' when it gave none. */
export function formatSourceTime(occurredAt?: string | null): string {
  if (!occurredAt) return '';
  const when = new Date(occurredAt);
  if (Number.isNaN(when.getTime())) return '';
  return when.toLocaleTimeString('en-IN', {
    hour: 'numeric',
    minute: '2-digit',
    timeZone: BANK_TIME_ZONE,
  });
}

/** The time of day of the first source that gave one. */
export function transactionTime(sources: TransactionSource[]): string {
  for (const source of sources) {
    const time = formatSourceTime(source.occurredAt);
    if (time) return time;
  }
  return '';
}
