import { useMemo, useState } from 'react';
import { ArrowDown, ArrowUp, CaretRight, CheckCircle, Target, Warning } from '@phosphor-icons/react';
import type { Category, CategoryGroup } from '../types/category.types';
import styles from './MonthSummary.module.css';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { toast } from '@/utils';
import { selectInflowCategory, updateCategoryBudget } from '../store/categorySlice';
import { getGoalProgress } from '../goals/goalProgress';
import { getLocaleDate, getPreviousMonthKey } from '@/utils/date.utils';
import { formatBudgetAmount } from './budgetFormat';

interface MonthSummaryProps {
  groups: CategoryGroup[];
  month: string;
  onSelectCategory: (category: Category) => void;
}

const TOP_SPENDING_LIMIT = 5;
const ATTENTION_LIMIT = 4;

const spentIn = (category: Category, month: string) => Math.max(-(category.activity?.[month] ?? 0), 0);

/** Shown in the inspector column while no category is selected. */
export function MonthSummary({ groups, month, onSelectCategory }: MonthSummaryProps) {
  const dispatch = useAppDispatch();
  const inflowCategory = useAppSelector(selectInflowCategory);
  const previousMonth = getPreviousMonthKey(month);
  const [assigning, setAssigning] = useState(false);

  const summary = useMemo(() => {
    // System groups hold the inflow category: its activity is income, not spending.
    const categories = groups
      .filter((group) => !group.isSystem)
      .flatMap((group) => group.categories ?? [])
      .filter((category) => category.id !== inflowCategory?.id);

    let assigned = 0;
    let spent = 0;
    let previousSpent = 0;
    let fundedCount = 0;
    for (const category of categories) {
      const budgeted = category.budgeted?.[month] ?? 0;
      assigned += budgeted;
      spent += spentIn(category, month);
      previousSpent += spentIn(category, previousMonth);
      if (budgeted > 0) fundedCount += 1;
    }

    const overspent = categories
      .filter((category) => (category.balance?.[month] ?? 0) < 0)
      .sort((a, b) => (a.balance?.[month] ?? 0) - (b.balance?.[month] ?? 0));
    const topSpending = categories
      .map((category) => ({ category, spent: spentIn(category, month) }))
      .filter((entry) => entry.spent > 0)
      .sort((a, b) => b.spent - a.spent)
      .slice(0, TOP_SPENDING_LIMIT);

    const underfunded = categories
      .map((category) => ({ category, progress: getGoalProgress(category, month) }))
      .filter(({ progress }) => progress && progress.status !== 'snoozed' && progress.underfunded > 0)
      .map(({ category, progress }) => ({ category, amount: progress!.underfunded }));
    const goalCount = categories.filter((category) => getGoalProgress(category, month)).length;

    return {
      assigned,
      spent,
      previousSpent,
      fundedCount,
      total: categories.length,
      overspent,
      topSpending,
      underfunded,
      goalCount,
    };
  }, [groups, month, previousMonth, inflowCategory?.id]);

  const income = Math.max(inflowCategory?.activity?.[month] ?? 0, 0);
  const monthName = getLocaleDate(month, { month: 'long' });
  const previousMonthName = getLocaleDate(previousMonth, { month: 'long' });
  const { assigned, spent, previousSpent, overspent, topSpending, underfunded, goalCount } = summary;
  const underfundedTotal = underfunded.reduce((sum, entry) => sum + entry.amount, 0);

  // YNAB's "Auto-Assign → Underfunded": tops every target up to what this
  // month needs. One request per category, in sequence, so each carryover
  // update lands before the next.
  const assignUnderfunded = async () => {
    setAssigning(true);
    let failed = 0;
    for (const { category, amount } of underfunded) {
      const budgeted = (category.budgeted?.[month] ?? 0) + amount;
      try {
        await dispatch(updateCategoryBudget({ budgeted, categoryId: category.id ?? '', month })).unwrap();
      } catch {
        failed += 1;
      }
    }
    setAssigning(false);
    if (failed) toast.error(`Couldn't assign to ${failed} ${failed === 1 ? 'category' : 'categories'}`);
    else toast.success(`Assigned ${formatBudgetAmount(underfundedTotal)} to ${underfunded.length} targets`);
  };
  const percentUsed = assigned > 0 ? Math.min((spent / assigned) * 100, 100) : spent > 0 ? 100 : 0;
  const change = previousSpent > 0 ? ((spent - previousSpent) / previousSpent) * 100 : null;
  const largestSpend = topSpending[0]?.spent ?? 0;

  return (
    <aside className={styles.summary} aria-label={`${monthName} summary`}>
      <header className={styles.heading}>
        <span className={styles.eyebrow}>Month summary</span>
        <h2 className={styles.title}>{getLocaleDate(month, { month: 'long', year: 'numeric' })}</h2>
      </header>

      <section className={styles.hero}>
        <span className={styles.label}>Spent so far</span>
        <strong className={styles.heroAmount}>{formatBudgetAmount(spent)}</strong>
        {(assigned > 0 || spent > 0) && (
          <div className={styles.progress}>
            <div className={styles.meter} aria-hidden="true">
              <div
                className={`${styles.meterFill} ${spent > assigned ? styles.meterOver : ''}`}
                style={{ width: `${percentUsed}%` }}
              />
            </div>
            <span className={styles.progressText}>
              {assigned > 0
                ? `${Math.round((spent / assigned) * 100)}% of ${formatBudgetAmount(assigned)} assigned`
                : 'Nothing assigned yet'}
            </span>
          </div>
        )}
        {change !== null && (
          <span className={`${styles.trend} ${change > 0 ? styles.trendUp : styles.trendDown}`}>
            {change > 0 ? <ArrowUp size={13} weight="bold" /> : <ArrowDown size={13} weight="bold" />}
            {Math.abs(Math.round(change))}% vs {previousMonthName}
            <span className={styles.trendMuted}>({formatBudgetAmount(previousSpent)})</span>
          </span>
        )}
      </section>

      <dl className={styles.tiles}>
        <div className={styles.tile}>
          <dt>Income</dt>
          <dd>{formatBudgetAmount(income)}</dd>
        </div>
        <div className={styles.tile}>
          <dt>Funded</dt>
          <dd>
            {summary.fundedCount}
            <span className={styles.tileMuted}> / {summary.total}</span>
          </dd>
        </div>
      </dl>

      {goalCount > 0 && (
        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Targets</h3>
          {underfunded.length === 0 ? (
            <p className={styles.allClear}>
              <CheckCircle size={16} weight="fill" />
              All {goalCount} targets funded
            </p>
          ) : (
            <>
              <p className={styles.targetsLine}>
                <Target size={15} weight="bold" className={styles.targetIcon} />
                <span>
                  {formatBudgetAmount(underfundedTotal)} underfunded across {underfunded.length} of {goalCount}{' '}
                  targets
                </span>
              </p>
              <button
                type="button"
                className={styles.assignButton}
                onClick={assignUnderfunded}
                disabled={assigning}>
                {assigning ? 'Assigning…' : `Assign ${formatBudgetAmount(underfundedTotal)}`}
              </button>
            </>
          )}
        </section>
      )}

      <section className={styles.section}>
        <h3 className={styles.sectionTitle}>Needs attention</h3>
        {overspent.length === 0 ? (
          <p className={styles.allClear}>
            <CheckCircle size={16} weight="fill" />
            No overspent categories
          </p>
        ) : (
          <ul className={styles.list}>
            {overspent.slice(0, ATTENTION_LIMIT).map((category) => (
              <li key={category.id}>
                <button type="button" className={styles.row} onClick={() => onSelectCategory(category)}>
                  <Warning size={15} weight="fill" className={styles.warnIcon} />
                  <span className={styles.rowName}>{category.name}</span>
                  <span className={`${styles.rowAmount} ${styles.negative}`}>
                    {formatBudgetAmount(category.balance?.[month] ?? 0)}
                  </span>
                  <CaretRight size={12} className={styles.rowCaret} />
                </button>
              </li>
            ))}
            {overspent.length > ATTENTION_LIMIT && (
              <li className={styles.more}>+{overspent.length - ATTENTION_LIMIT} more overspent</li>
            )}
          </ul>
        )}
      </section>

      <section className={styles.section}>
        <h3 className={styles.sectionTitle}>Top spending</h3>
        {topSpending.length === 0 ? (
          <p className={styles.empty}>No spending in {monthName} yet</p>
        ) : (
          <ul className={styles.list}>
            {topSpending.map(({ category, spent: categorySpent }) => (
              <li key={category.id}>
                <button
                  type="button"
                  className={`${styles.row} ${styles.spendRow}`}
                  onClick={() => onSelectCategory(category)}>
                  <span className={styles.rowName}>{category.name}</span>
                  <span className={styles.rowAmount}>{formatBudgetAmount(categorySpent)}</span>
                  <span className={styles.bar} aria-hidden="true">
                    <span style={{ width: `${(categorySpent / largestSpend) * 100}%` }} />
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <p className={styles.hint}>Select a category to see its details</p>
    </aside>
  );
}
