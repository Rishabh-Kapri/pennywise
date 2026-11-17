/** Mirrors backend/shared/model/statement.go. */

export type StatementDirection = 'debit' | 'credit';
export type StatementImportStatus = 'processing' | 'ready' | 'failed';
export type StatementMatchKind = 'exact' | 'amount_differs';

export interface StatementRow {
  id: string;
  date: string;
  /** "HH:MM" printed with the date, in the bank's zone */
  time?: string;
  description: string;
  /** model's cleaned merchant name, the default payee on create */
  payee: string;
  /** always positive; see direction */
  amount: number;
  direction: StatementDirection;
  balance?: number;
  reference?: string;
  page?: number;
  /** the row's line in a CSV export, numbered as StatementCsvPanel shows them */
  line?: number;
  /** the running balance contradicted the model and the API flipped it */
  directionCorrected?: boolean;
  /** points the card statement printed for this row; negative for a reversal */
  rewardPoints?: number;
  /** what the statement printed, when description is the cleaned text */
  rawDescription?: string;
  channel?: string;
  merchant?: string;
  vpa?: string;
  valueDate?: string;
  /** the value date is more than a couple of days from the row's date */
  valueDateMismatch?: boolean;
  /** set for the bank's own charges */
  chargeKind?: 'tax' | 'interest' | 'fee';
  /** set for a card EMI line */
  emi?: { part: 'instalment' | 'principal' | 'interest' | 'conversion'; number?: number; of?: number };
  /** the user's word on "already-paid EMI": false moved it out of Covered, true set it aside */
  emiOverride?: boolean;
  /** a category for a missing row: what an earlier statement's same line got, or by name for a charge */
  suggestedCategoryId?: string;
  /** the payee an earlier statement's same line was given */
  suggestedPayeeName?: string;
  /** what was read before the user corrected the row */
  edited?: { date: string; time?: string; description: string; amount: number; direction: 'debit' | 'credit' };
  /** transactions the user linked to this row by hand */
  linkedTransactionIds?: string[];
}

export interface StatementEMIPlan {
  description: string;
  bookedOn?: string;
  principal?: number;
  monthlyAmount?: number;
  interestRate?: number;
  tenureMonths?: number;
  remainingMonths?: number;
  outstanding?: number;
}

export interface StatementChargeTotal {
  kind: 'tax' | 'interest' | 'fee';
  amount: number;
  count: number;
}

export interface StatementSectionSummary {
  accountLabel: string;
  accountNumber: string;
  kind?: 'card' | 'bank';
  currency?: string;
  periodStart?: string;
  periodEnd?: string;
  statementDate?: string;
  closingBalance?: number;
  dueDate?: string;
  totalDue?: number;
  minimumDue?: number;
  accountId?: string;
}

export interface StatementImport {
  id: string;
  budgetId: string;
  fileName: string;
  status: StatementImportStatus;
  error?: string;
  /** The statement.extract job reading it; live progress comes from its updates. */
  jobId?: string;
  /** What the job is doing now ("rendering pages", "reading pages"). */
  stage?: string;
  pageCount: number;
  pagesProcessed: number;
  defaultAccountId?: string;
  droppedRows: number;
  rowCount: number;
  /** The uploaded PDF is kept (GET statements/:id/file); false for older imports. */
  hasFile?: boolean;
  /** Accounts its sections are mapped to (list only). */
  accountIds?: string[];
  /** Each section without its rows (list only): whose statement, for when, what's due. */
  summary?: StatementSectionSummary[];
  /** where its rows stand (list only, ready imports) */
  progress?: StatementProgress;
  createdAt: string;
  updatedAt: string;
}

export interface StatementLedgerTxn {
  id: string;
  date: string;
  amount: number;
  payeeName: string;
  categoryName?: string;
  note: string;
  status: string;
}

export interface StatementMatch {
  row: StatementRow;
  transaction: StatementLedgerTxn;
  kind: StatementMatchKind;
  dateDiffDays: number;
  /** paired by the user (row.linkedTransactionIds) */
  linked?: boolean;
  /** further transactions linked to the row, e.g. a booked difference */
  others?: StatementLedgerTxn[];
  /** further rows linked to the same transactions (EMI lines one payment paid) */
  otherRows?: StatementRow[];
  /** row amount minus the linked transactions', while they don't add up */
  difference?: number;
}

export interface StatementDifferenceRequest {
  payeeName?: string;
  categoryId?: string;
}

export interface StatementBalanceCheck {
  computed: number;
  reported: number;
  difference: number;
  ok: boolean;
}

export interface StatementRewards {
  opening?: number;
  earned?: number;
  redeemed?: number;
  expired?: number;
  closing?: number;
}

export interface StatementSectionReconciliation {
  index: number;
  accountLabel: string;
  /** as printed, usually masked */
  accountNumber: string;
  kind?: 'card' | 'bank';
  currency?: string;
  accountId?: string;
  accountName?: string;
  periodStart: string;
  periodEnd: string;
  openingBalance?: number;
  closingBalance?: number;
  balanceCheck?: StatementBalanceCheck;
  statementDate?: string;
  dueDate?: string;
  totalDue?: number;
  minimumDue?: number;
  creditLimit?: number;
  rewards?: StatementRewards;
  emiPlans?: StatementEMIPlan[];
  rewardPointsOnRows?: number;
  charges?: StatementChargeTotal[];
  /** Cashback credited on the statement, net of reversals. */
  cashback?: StatementChargeTotal;
  rowCount: number;
  matched: StatementMatch[];
  missing: StatementRow[];
  ledgerOnly: StatementLedgerTxn[];
  /** EMI instalments: the purchase is already in the budget */
  covered: StatementRow[];
}

export interface StatementReconciliation {
  import: StatementImport;
  sections: StatementSectionReconciliation[];
}

export interface StatementUploadResponse {
  import: StatementImport;
  duplicate: boolean;
}

export interface StatementCreateItem {
  rowId: string;
  payeeName?: string;
  categoryId?: string;
  note?: string;
}

export interface StatementRowEdit {
  date?: string;
  /** "HH:MM"; '' removes it */
  time?: string;
  description?: string;
  amount?: number;
  direction?: 'debit' | 'credit';
}

export interface StatementProgress {
  missing: number;
  matched: number;
  ledgerOnly: number;
  covered: number;
  unmapped: number;
  reconciled: boolean;
}

export interface StatementCreateResult {
  created: string[];
  skipped: string[];
  failed: { rowId: string; error: string }[];
}

/** signed in ledger convention: inflows positive */
export const signedAmount = (row: StatementRow) => (row.direction === 'credit' ? row.amount : -row.amount);
