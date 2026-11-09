import type { Account } from '@/features/accounts/types/account.types';
import type { Payee } from '@/features/payees/types/payee.types';
import type { TransactionPrefill } from '@/features/transactions/types/filter.types';
import type { JobDetail } from './types/job.types';

interface StepPayload {
  entities?: { account?: string; merchant?: string; amount?: number; date?: string };
  verdict?: { type?: string };
  message?: { receivedAt?: string; subject?: string };
  receivedAt?: string;
  subject?: string;
}

/** The part of a timestamp the form uses. Go's zero time means no date was read. */
function dateOnly(value?: string): string | undefined {
  return value && !value.startsWith('0001') ? value.slice(0, 10) : undefined;
}

/** What a stopped or failed message already worked out, for the add-transaction form. */
export function messagePrefill(detail: JobDetail, accounts: Account[], payees: Payee[]): TransactionPrefill {
  const payload = (detail.payload ?? {}) as StepPayload;
  const { entities, verdict } = payload;
  const message = payload.message ?? payload;
  const prefill: TransactionPrefill = {};

  const date = dateOnly(entities?.date) ?? dateOnly(message.receivedAt);
  if (date) prefill.date = date;

  const amount = entities?.amount ?? 0;
  if (amount !== 0) {
    const inflow = verdict?.type === 'credit' || (verdict?.type !== 'debit' && amount > 0);
    if (inflow) prefill.inflow = Math.abs(amount);
    else prefill.outflow = Math.abs(amount);
  }

  // The same match the pipeline makes: the first run of digits against the account's suffix.
  const suffix = entities?.account?.match(/\d+/)?.[0];
  const account = suffix ? accounts.find((candidate) => candidate.suffix === suffix) : undefined;
  if (account?.id) {
    prefill.accountId = account.id;
    prefill.accountName = account.name;
  }

  const merchant = entities?.merchant?.trim();
  const payee = merchant
    ? payees.find((candidate) => candidate.name?.toLowerCase() === merchant.toLowerCase())
    : undefined;
  if (payee?.id) {
    prefill.payeeId = payee.id;
    prefill.payeeName = payee.name;
  }
  const note = payee ? '' : merchant || message.subject || '';
  if (note) prefill.note = note;
  return prefill;
}
