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

