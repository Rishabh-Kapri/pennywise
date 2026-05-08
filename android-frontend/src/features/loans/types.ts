import type { LoadingState } from '../../utils/constants';

export interface LoanMetadata {
  accountId: string;
  interestRate: number;
  originalBalance: number;
  monthlyPayment: number;
  loanStartDate: string;
  categoryId?: string;
}

export interface LoanState {
  loanMetadata: Record<string, LoanMetadata>;
  loading: LoadingState;
  error: string | null;
}
