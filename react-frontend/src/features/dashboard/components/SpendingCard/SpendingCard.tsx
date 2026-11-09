import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, ChartDonut, Warning, WarningCircle } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import type { RootState } from '@/app/store';
import { selectCashFlowRefreshing, selectSpendingSummary, selectUncategorizedCount } from '../../store/dashboardSlice';
import type { SpendingCategory } from '../../types';
import { formatCurrency, formatMonthName } from '../../utils';
import { Amount } from '../Amount';
import { DashboardCard } from '../DashboardCard';
import styles from './SpendingCard.module.css';

const RING_RADIUS = 27;

/** Spent against what was assigned this month. */
function CategoryRing({ category }: { category: SpendingCategory }) {
  const budgeted = category.budgeted > 0;
  const over = budgeted && category.spent > category.budgeted;
  const progress = budgeted ? Math.min(category.spent / category.budgeted, 1) : 0;
  const state = over ? 'over' : budgeted ? 'ok' : 'unbudgeted';

  return (
    <li className={styles.ringItem} data-state={state}>
      <span className={styles.ring}>
        <svg className={styles.ringGraphic} viewBox="0 0 64 64" aria-hidden="true">
          <circle className={styles.ringTrack} cx="32" cy="32" r={RING_RADIUS} />
          {budgeted && (
            <circle
              className={styles.ringValue}
              cx="32"
              cy="32"
              r={RING_RADIUS}
              pathLength={100}
              strokeDasharray={`${progress * 100} 100`}
              transform="rotate(-90 32 32)"
            />
          )}
        </svg>
        {budgeted && (
          <span className={styles.ringPercent} aria-hidden="true">
            {Math.round((category.spent / category.budgeted) * 100)}%
          </span>
        )}
      </span>
      <span className={styles.ringName} title={category.name}>
        {category.name}
      </span>
      <span className={styles.ringAmount}>{formatCurrency(category.spent)}</span>
      <span className={styles.ringStatus}>
        {over && <Warning size={12} weight="bold" aria-hidden="true" />}
        {over
          ? `${formatCurrency(category.spent - category.budgeted)} over`
          : !budgeted
            ? 'Not budgeted'
            : category.spent === category.budgeted
              ? 'Fully spent'
              : `${formatCurrency(category.budgeted - category.spent)} left`}
      </span>
    </li>
  );
}

export default function SpendingCard({ className = '' }: { className?: string }) {
  const summary = useAppSelector(selectSpendingSummary);
  const refreshing = useAppSelector(selectCashFlowRefreshing);
  const month = summary?.month ?? '';
  const uncategorized = useAppSelector((state: RootState) => selectUncategorizedCount(state, month));
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const groups = summary?.groups ?? [];
  const selected = groups.find((group) => group.id === selectedId) ?? groups[0];
  const monthName = month ? formatMonthName(month) : '';

  return (
    <DashboardCard
      title="Spending"
      icon={<ChartDonut size={18} />}
      className={className}
      refreshing={refreshing}
      actions={
        summary &&
        summary.total > 0 && (
          <span className={styles.headerTotal}>
            <strong>{formatCurrency(summary.total)}</strong>
            {summary.budgeted > 0 && ` of ${formatCurrency(summary.budgeted)} assigned`}
          </span>
        )
      }
    >
      {!selected ? (
        <p className={styles.empty}>{monthName ? `No spending in ${monthName} yet.` : 'No spending yet.'}</p>
      ) : (
        <div className={styles.body}>
          <ul className={styles.groupList} aria-label="Category groups">
            {groups.map((group) => {
              const share = summary && summary.total > 0 ? (group.spent / summary.total) * 100 : 0;
              const isSelected = group.id === selected.id;
              return (
                <li key={group.id}>
                  <button
                    type="button"
                    className={styles.groupRow}
                    aria-pressed={isSelected}
                    onClick={() => setSelectedId(group.id)}
                  >
                    <span className={styles.groupTop}>
                      <span className={styles.groupName}>{group.name}</span>
                      <span className={styles.groupAmount}>{formatCurrency(group.spent)}</span>
                    </span>
                    <span className={styles.groupBar} aria-hidden="true">
                      <span className={styles.groupFill} style={{ width: `${Math.max(share, 1.5)}%` }} />
                    </span>
                    <span className={styles.groupShare}>{Math.round(share)}% of spending</span>
                  </button>
                </li>
              );
            })}
          </ul>

          <div className={styles.detail} aria-live="polite">
            <div className={styles.detailHead}>
              <h3 className={styles.detailTitle}>{selected.name}</h3>
              <Amount value={selected.spent} className={styles.detailAmount} />
              <p className={styles.detailMeta}>
                {selected.budgeted > 0
                  ? `of ${formatCurrency(selected.budgeted)} assigned`
                  : 'Nothing assigned this month'}
                {' · '}
                {selected.categories.length} {selected.categories.length === 1 ? 'category' : 'categories'}
              </p>
            </div>
            <ul className={styles.rings}>
              {selected.categories.map((category) => (
                <CategoryRing key={category.id} category={category} />
              ))}
            </ul>
          </div>
        </div>
      )}

      {uncategorized > 0 && (
        <Link to="/transactions" className={styles.attention}>
          <WarningCircle size={16} weight="bold" aria-hidden="true" />
          {uncategorized} uncategorized {uncategorized === 1 ? 'transaction' : 'transactions'} in {monthName}
          <ArrowRight size={14} className={styles.attentionArrow} aria-hidden="true" />
        </Link>
      )}
    </DashboardCard>
  );
}
