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
}

export interface StatementImport {
  id: string;
  budgetId: string;
  fileName: string;
  status: StatementImportStatus;
  error?: string;
  pageCount: number;
  pagesProcessed: number;
  defaultAccountId?: string;
  droppedRows: number;
  rowCount: number;
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

export interface StatementSectionReconciliation {
  index: number;
  accountLabel: string;
  accountNumber: string;
  currency?: string;
  accountId?: string;
  accountName?: string;
  periodStart: string;
  periodEnd: string;
  openingBalance?: number;
  closingBalance?: number;
  balanceCheck?: StatementBalanceCheck;
  rowCount: number;
  matched: StatementMatch[];
  missing: StatementRow[];
  ledgerOnly: StatementLedgerTxn[];
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
