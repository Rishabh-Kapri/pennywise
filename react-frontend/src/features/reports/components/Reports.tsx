import { useEffect, useState } from 'react';
import { FunnelSimple } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { getSelectedMonthInHumanFormat } from '@/utils/date.utils';
import {
  fetchCashFlowReport,
  fetchNetWorthReport,
  selectCashFlowLoading,
  selectNetWorthLoading,
  selectReportError,
  selectReportFilters,
  selectReportRange,
} from '../store/reportSlice';
import type { ReportTab } from '../types/report.types';
import ReportFilterPanel from './ReportFilterPanel';
import ReportRangePicker from './ReportRangePicker';
import OverviewReport from './OverviewReport';
import SpendingReport from './SpendingReport';
import IncomeReport from './IncomeReport';
import InvestingReport from './InvestingReport';
import NetworthReport from './NetworthReport';
import styles from './Reports.module.css';

const TABS: { key: ReportTab; label: string }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'spending', label: 'Spending' },
  { key: 'income', label: 'Income' },
  { key: 'investing', label: 'Investing' },
  { key: 'networth', label: 'Net worth' },
];

export default function Reports() {
  const dispatch = useAppDispatch();
  const [activeTab, setActiveTab] = useState<ReportTab>('overview');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const range = useAppSelector(selectReportRange);
  const filters = useAppSelector(selectReportFilters);
  const cashFlowLoading = useAppSelector(selectCashFlowLoading);
  const netWorthLoading = useAppSelector(selectNetWorthLoading);
  const error = useAppSelector(selectReportError);
  // on a fresh load the budget arrives with fetchAllBudgets, after this
  // mounts; fetching earlier sends no x-budget-id and the error sticks
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);
  const filterCount = filters.accountIds.length + filters.categoryIds.length + filters.tagIds.length;

  useEffect(() => {
    // setRange/setReportFilters reset each report's loading state to IDLE,
    // so a range or filter change (or first visit) refetches only what the
    // visible tab needs; the four cash-flow tabs share one response
    if (!budgetId) return;
    if (activeTab === 'networth') {
      if (netWorthLoading === LoadingState.IDLE) dispatch(fetchNetWorthReport({ range, filters }));
    } else if (cashFlowLoading === LoadingState.IDLE) {
      dispatch(fetchCashFlowReport({ range, filters }));
    }
  }, [dispatch, activeTab, range, filters, cashFlowLoading, netWorthLoading, budgetId]);

  return (
    <div className={styles.container}>
      <div className={styles.inner}>
        <header className={styles.header}>
          <div className={styles.titleBlock}>
            <h1 className={styles.title}>Reports</h1>
            <p className={styles.subtitle}>
              {getSelectedMonthInHumanFormat(range.startMonth)} – {getSelectedMonthInHumanFormat(range.endMonth)}
            </p>
          </div>
          <div className={styles.controls}>
            <ReportRangePicker />
            <button
              type="button"
              className={styles.filterToggle}
              aria-expanded={filtersOpen}
              onClick={() => setFiltersOpen((open) => !open)}
            >
              <FunnelSimple size={16} />
              Filters
              {filterCount > 0 && <span className={styles.filterCount}>{filterCount}</span>}
            </button>
          </div>
        </header>

        {filtersOpen && (
          <div className={styles.filterPanel}>
            <ReportFilterPanel />
          </div>
        )}

        <nav className={styles.tabs} aria-label="Report">
          {TABS.map((tab) => (
            <button
              key={tab.key}
              type="button"
              className={styles.tab}
              aria-current={activeTab === tab.key ? 'page' : undefined}
              onClick={() => setActiveTab(tab.key)}
            >
              {tab.label}
            </button>
          ))}
        </nav>

        {error && <div className={styles.error}>{error}</div>}

        {activeTab === 'overview' && <OverviewReport />}
        {activeTab === 'spending' && <SpendingReport />}
        {activeTab === 'income' && <IncomeReport />}
        {activeTab === 'investing' && <InvestingReport />}
        {activeTab === 'networth' && <NetworthReport />}
      </div>
    </div>
  );
}
