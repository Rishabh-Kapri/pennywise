/** Mirrors backend/shared/model/statement.go. */

export type StatementDirection = 'debit' | 'credit';
export type StatementImportStatus = 'processing' | 'ready' | 'failed';
export type StatementMatchKind = 'exact' | 'amount_differs';

export interface StatementRow {
  id: string;
  date: string;
  description: string;
  /** model's cleaned merchant name, the default payee on create */
  payee: string;
  /** always positive; see direction */
  amount: number;
  direction: StatementDirection;
  balance?: number;
  reference?: string;
  page?: number;
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
  /** a category for a missing charge, picked by name */
  suggestedCategoryId?: string;
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
}

export interface StatementCreateResult {
  created: string[];
  skipped: string[];
  failed: { rowId: string; error: string }[];
}

/** signed in ledger convention: inflows positive */
export const signedAmount = (row: StatementRow) =>
  row.direction === 'credit' ? row.amount : -row.amount;
