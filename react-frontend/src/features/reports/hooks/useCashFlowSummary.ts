import { useMemo } from 'react';
import { useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { summarizeCashFlow, type CashFlowSummary } from '../cashFlow';
import { selectCashFlowLoading, selectCashFlowReport, selectGroupTreatments } from '../store/reportSlice';

/** The cash-flow report, classified with the user's group treatments. */
export function useCashFlowSummary(): { summary: CashFlowSummary | null; loading: boolean } {
  const report = useAppSelector(selectCashFlowReport);
  const loading = useAppSelector(selectCashFlowLoading);
  const treatments = useAppSelector(selectGroupTreatments);

  const summary = useMemo(() => (report ? summarizeCashFlow(report, treatments) : null), [report, treatments]);

  return { summary, loading: loading === LoadingState.PENDING || loading === LoadingState.IDLE };
}
