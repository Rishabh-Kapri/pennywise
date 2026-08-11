import type { LoadingState } from '@/utils';

export interface PredictionReviewItem {
  id: string;
  budgetId: string;
  transactionId: string;
  emailText?: string | null;
  llmReasoning?: string | null;
  amount?: number | null;
  extractedAccount?: string | null;
  extractedPayee?: string | null;
  predictedPayeeId?: string | null;
  predictedCategoryId?: string | null;
  accountConfidence?: number | null;
  payeeConfidence?: number | null;
  categoryConfidence?: number | null;
  source: string;
  hasUserCorrected: boolean;
  actualPayeeId?: string | null;
  actualCategoryId?: string | null;
  reviewedAt?: string | null;
  createdAt: string;

  transactionDate: string;
  transactionAmount: number;
  transactionNote: string;
  accountName?: string | null;
  currentPayeeId?: string | null;
  currentPayeeName?: string | null;
  currentCategoryId?: string | null;
  currentCategoryName?: string | null;
  predictedPayeeName?: string | null;
  predictedCategoryName?: string | null;
}

export type PredictionReviewAction = 'accept' | 'correct';

export interface PredictionReviewRequest {
  action: PredictionReviewAction;
  payeeId?: string | null;
  categoryId?: string | null;
}

export interface PredictionReviewState {
  items: PredictionReviewItem[];
  loading: LoadingState;
  includeReviewed: boolean;
  submittingId: string | null;
  error: string | null;
}

/** Confidence buckets drive the chip colour and the "needs a look" ordering. */
export type ConfidenceLevel = 'high' | 'medium' | 'low' | 'unknown';

export function confidenceLevel(value?: number | null): ConfidenceLevel {
  if (value === null || value === undefined) return 'unknown';
  const percent = value <= 1 ? value * 100 : value;
  if (percent >= 80) return 'high';
  if (percent >= 50) return 'medium';
  return 'low';
}

export function formatConfidence(value?: number | null): string {
  if (value === null || value === undefined) return '—';
  const percent = value <= 1 ? value * 100 : value;
  return `${percent.toFixed(0)}%`;
}
