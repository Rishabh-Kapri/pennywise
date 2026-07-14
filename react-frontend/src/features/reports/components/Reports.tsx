import { useEffect, useState } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import {
  fetchIncomeExpenseReport,
  fetchNetWorthReport,
  fetchSpendingReport,
  selectIncomeExpenseLoading,
  selectNetWorthLoading,
  selectReportError,
  selectReportRange,
  selectSpendingLoading,
} from '../store/reportSlice';
import type { ReportTab } from '../types/report.types';
import ReportRangePicker from './ReportRangePicker';
import SpendingReport from './SpendingReport';
import IncomeExpenseReport from './IncomeExpenseReport';
import NetworthReport from './NetworthReport';
import styles from './Reports.module.css';

const TABS: { key: ReportTab; label: string }[] = [
  { key: 'spending', label: 'Spending' },
  { key: 'incomeExpense', label: 'Income vs Expense' },
  { key: 'networth', label: 'Net Worth' },
];

export default function Reports() {
  const dispatch = useAppDispatch();
  const [activeTab, setActiveTab] = useState<ReportTab>('spending');
  const range = useAppSelector(selectReportRange);
  const spendingLoading = useAppSelector(selectSpendingLoading);
  const incomeExpenseLoading = useAppSelector(selectIncomeExpenseLoading);
  const netWorthLoading = useAppSelector(selectNetWorthLoading);
  const error = useAppSelector(selectReportError);

  useEffect(() => {
    // setRange resets each report's loading state to IDLE, so a range
    // change (or first visit) refetches only the visible tab
    if (activeTab === 'spending' && spendingLoading === LoadingState.IDLE) {
      dispatch(fetchSpendingReport(range));
    } else if (activeTab === 'incomeExpense' && incomeExpenseLoading === LoadingState.IDLE) {
      dispatch(fetchIncomeExpenseReport(range));
    } else if (activeTab === 'networth' && netWorthLoading === LoadingState.IDLE) {
      dispatch(fetchNetWorthReport(range));
    }
  }, [dispatch, activeTab, range, spendingLoading, incomeExpenseLoading, netWorthLoading]);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>Reports</h1>
        <ReportRangePicker />
      </div>

      <div className={styles.tabs}>
        {TABS.map((tab) => (
          <button
            key={tab.key}
            className={activeTab === tab.key ? styles.tabActive : styles.tab}
            onClick={() => setActiveTab(tab.key)}>
            {tab.label}
          </button>
        ))}
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {activeTab === 'spending' && <SpendingReport />}
      {activeTab === 'incomeExpense' && <IncomeExpenseReport />}
      {activeTab === 'networth' && <NetworthReport />}
    </div>
  );
}
