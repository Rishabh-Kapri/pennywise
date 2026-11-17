/** Mirrors backend/shared/model/statement.go (and react-frontend's reconcile.types.ts). */

export type StatementDirection = 'debit' | 'credit';
export type StatementImportStatus = 'processing' | 'ready' | 'failed';
export type StatementMatchKind = 'exact' | 'amount_differs';

export interface StatementRow {
  id: string;
  date: string;
  description: string;
  /** the reader's cleaned merchant name, the default payee on create */
  payee: string;
  /** always positive; see direction */
  amount: number;
  direction: StatementDirection;
  balance?: number;
  reference?: string;
  /** the PDF page the row was read from */
  page?: number;
  rawDescription?: string;
  rewardPoints?: number;
  directionCorrected?: boolean;
  chargeKind?: 'tax' | 'interest' | 'fee';
  /** a category for a missing charge, picked by name */
  suggestedCategoryId?: string;
  linkedTransactionIds?: string[];
  /** the user's word on "already-paid EMI": false moved it out of Covered, true set it aside */
  emiOverride?: boolean;
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
  /** paired by the user */
  linked?: boolean;
  /** further transactions linked to the row, e.g. a booked difference */
  others?: StatementLedgerTxn[];
  /** row amount minus the linked transactions', while they don't add up */
  difference?: number;
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

export interface StatementChargeTotal {
  kind: string;
  amount: number;
  count: number;
}

export interface StatementSectionReconciliation {
  index: number;
  accountLabel: string;
  accountNumber: string;
  kind?: 'card' | 'bank';
  currency?: string;
  accountId?: string;
  accountName?: string;
  periodStart: string;
  periodEnd: string;
  balanceCheck?: StatementBalanceCheck;
  dueDate?: string;
  totalDue?: number;
  minimumDue?: number;
  creditLimit?: number;
  rewards?: StatementRewards;
  rewardPointsOnRows?: number;
  charges?: StatementChargeTotal[];
  cashback?: StatementChargeTotal;
  rowCount: number;
  matched: StatementMatch[];
  missing: StatementRow[];
  ledgerOnly: StatementLedgerTxn[];
  /** EMI instalments: the purchase is already in the budget */
  covered: StatementRow[];
}

/** A section without its rows, as the list returns it. */
export interface StatementSectionSummary {
  accountLabel: string;
  accountNumber: string;
  kind?: 'card' | 'bank';
  periodStart?: string;
  periodEnd?: string;
  dueDate?: string;
  totalDue?: number;
  accountId?: string;
}

export interface StatementImport {
  id: string;
  fileName: string;
  status: StatementImportStatus;
  error?: string;
  jobId?: string;
  stage?: string;
  pageCount: number;
  pagesProcessed: number;
  droppedRows: number;
  rowCount: number;
  hasFile?: boolean;
  summary?: StatementSectionSummary[];
  createdAt: string;
}

export interface StatementReconciliation {
  import: StatementImport;
  sections: StatementSectionReconciliation[];
}

export interface StatementCreateItem {
  rowId: string;
  payeeName?: string;
  categoryId?: string;
}

export interface StatementCreateResult {
  created: string[];
  skipped: string[];
  failed: { rowId: string; error: string }[];
}

export interface StatementDifferenceRequest {
  payeeName?: string;
  categoryId?: string;
}

/** signed in ledger convention: inflows positive */
export const signedAmount = (row: StatementRow) => (row.direction === 'credit' ? row.amount : -row.amount);
