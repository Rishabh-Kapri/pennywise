/**
 * Banks we have logos for (files in public/bank-logos, provenance in its
 * README). An account stores its bank's id in `account.bank`; older accounts
 * have none, so the bank is read off the name: "HDFC Checking",
 * "ICICI Credit Card", "SBI Savings".
 */
export interface BankBrand {
  id: string;
  name: string;
  /** What people call it; seeds the account name ("HDFC Checking"). */
  short: string;
}

const BANKS: Array<BankBrand & { match: RegExp }> = [
  { id: 'hdfc', short: 'HDFC', name: 'HDFC Bank', match: /\bhdfc\b/i },
  { id: 'icici', short: 'ICICI', name: 'ICICI Bank', match: /\bicici\b/i },
  { id: 'sbi', short: 'SBI', name: 'State Bank of India', match: /\b(sbi|state bank)\b/i },
  { id: 'axis', short: 'Axis', name: 'Axis Bank', match: /\baxis\b/i },
  { id: 'kotak', short: 'Kotak', name: 'Kotak Mahindra Bank', match: /\bkotak\b/i },
  { id: 'pnb', short: 'PNB', name: 'Punjab National Bank', match: /\b(pnb|punjab national)\b/i },
  { id: 'canara', short: 'Canara', name: 'Canara Bank', match: /\bcanara\b/i },
];

export const BANK_BRANDS: BankBrand[] = BANKS.map(({ id, name, short }) => ({ id, name, short }));

/** The account's bank: its stored id if it has one, else guessed from its name. */
export function getBankBrand(accountName?: string | null, bankId?: string | null): BankBrand | null {
  if (bankId) {
    const stored = BANK_BRANDS.find((bank) => bank.id === bankId);
    if (stored) return stored;
  }
  if (!accountName) return null;
  return BANKS.find((bank) => bank.match.test(accountName)) ?? null;
}

export const bankLogoUrl = (bank: BankBrand, kind: 'mark' | 'full' = 'mark') =>
  `/bank-logos/${bank.id}${kind === 'mark' ? '-mark' : ''}.svg`;
