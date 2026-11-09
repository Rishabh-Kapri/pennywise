import { useRef, useState } from 'react';
import type { Category } from '../types/category.types';
import styles from './CategoryItemList.module.css';
import { MovePopover } from './popovers/MovePopover';
import { AmountCell } from './AmountCell';
import { CategoryActions } from './CategoryActions';
import { ActivityPopover } from './ActivityModal';
import { useAssignedEditor } from '../hooks/useAssignedEditor';
import { formatBudgetAmount } from './budgetFormat';
import { getGoalProgress, type GoalProgress } from '../goals/goalProgress';
import { describeProgress } from '../goals/goalText';

interface Props {
  month: string;
  categories: Category[];
  selectedCategoryId?: string;
  onSelectCategory: (category: Category) => void;
  openPopoverId: string | null;
  onPopoverOpen: (id: string) => void;
  onPopoverClose: () => void;
}

interface CategoryItemProps extends Omit<Props, 'categories'> {
  category: Category;
}

type MeterStatus = 'healthy' | 'warning' | 'danger';

function getMeterStatus(balance: number, percentUsed: number): MeterStatus {
  if (balance < 0) return 'danger';
  if (percentUsed >= 80) return 'warning';
  return 'healthy';
}

// A target's progress takes over the meter: how much of this month's need is
// funded, rather than how much of the assigned amount is spent.
const GOAL_STATUS: Record<GoalProgress['status'], MeterStatus | 'snoozed'> = {
  funded: 'healthy',
  underfunded: 'warning',
  overspent: 'danger',
  snoozed: 'snoozed',
};

const STATUS_CLASS: Record<MeterStatus | 'snoozed', string> = {
  healthy: styles.healthy,
  warning: styles.warning,
  danger: styles.danger,
  snoozed: styles.snoozed,
};

export function CategoryItem({
  month,
  category,
  selectedCategoryId,
  onSelectCategory,
  openPopoverId,
  onPopoverOpen,
  onPopoverClose,
}: CategoryItemProps) {
  const availableRef = useRef<HTMLButtonElement | null>(null);
  const activityRef = useRef<HTMLButtonElement | null>(null);
  const [showActivity, setShowActivity] = useState(false);
  const editor = useAssignedEditor(category, month);

  const assigned = editor.assigned;
  const activity = category.activity?.[month] ?? 0;
  const balance = category.balance?.[month] ?? 0;
  const spent = Math.max(-activity, 0);
  const percentUsed = assigned > 0 ? (spent / assigned) * 100 : spent > 0 ? 100 : 0;
  const goal = getGoalProgress(category, month);
  const showMeter = goal !== null || assigned > 0 || spent > 0;
  const meterStatus = goal ? GOAL_STATUS[goal.status] : getMeterStatus(balance, percentUsed);
  const meterPercent = goal ? goal.percent : Math.min(percentUsed, 100);
  const isSelected = selectedCategoryId === category.id;

  const pillClass = [
    styles.pill,
    balance < 0 && styles.pillOver,
    balance > 0 && styles.pillPositive,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div
      role="row"
      aria-selected={isSelected}
      onClick={() => onSelectCategory(category)}
      className={`${styles.row} ${isSelected ? styles.selected : ''}`}>
      <div className={styles.nameCell} role="cell">
        <div className={styles.nameLine}>
          <span className={styles.name}>{category.name}</span>
          <span className={styles.actions}>
            <CategoryActions category={category} />
          </span>
        </div>
        {showMeter && (
          <div className={styles.meterRow}>
            <div className={`${styles.meter} ${STATUS_CLASS[meterStatus]}`} aria-hidden="true">
              <div className={styles.meterFill} style={{ width: `${meterPercent}%` }} />
            </div>
            {goal && goal.status !== 'funded' && (
              <span className={`${styles.goalHint} ${STATUS_CLASS[meterStatus]}`}>{describeProgress(goal)}</span>
            )}
          </div>
        )}
        <span className={styles.mobileMeta}>
          {formatBudgetAmount(spent)} of {formatBudgetAmount(assigned)} spent
        </span>
      </div>

      <div className={`${styles.cell} ${styles.assignedCell}`} role="cell">
        {editor.isEditing ? (
          <AmountCell
            value={editor.draft}
            isEditing
            onChange={editor.onChange}
            onBlur={editor.commit}
            onKeyDown={editor.onKeyDown}
          />
        ) : (
          <button
            type="button"
            className={styles.cellButton}
            aria-label={`Edit amount assigned to ${category.name}`}
            onClick={(e) => {
              e.stopPropagation();
              editor.start();
            }}>
            <AmountCell value={assigned} />
          </button>
        )}
      </div>

      <div className={`${styles.cell} ${styles.activityCell}`} role="cell">
        <button
          ref={activityRef}
          type="button"
          className={styles.cellButton}
          aria-label={`Show ${category.name} transactions`}
          onClick={(e) => {
            e.stopPropagation();
            setShowActivity(true);
          }}>
          <AmountCell value={activity} />
        </button>
        <ActivityPopover
          isOpen={showActivity}
          onClose={() => setShowActivity(false)}
          triggerRef={activityRef}
          categoryId={category.id ?? ''}
          categoryName={category.name}
          month={month}
          activityAmount={activity}
        />
      </div>

      <div className={styles.cell} role="cell">
        <button
          ref={availableRef}
          type="button"
          className={pillClass}
          aria-haspopup="dialog"
          aria-controls={`popover-content-${category.id}`}
          aria-label={`Move money from ${category.name}`}
          onClick={(e) => {
            e.stopPropagation();
            onPopoverOpen(category.id ?? '');
          }}>
          {formatBudgetAmount(balance)}
        </button>
        <MovePopover
          triggerRef={availableRef}
          isOpen={openPopoverId === category.id}
          categoryId={category.id ?? ''}
          categoryName={category.name}
          amount={balance}
          onClose={onPopoverClose}
        />
      </div>
    </div>
  );
}

export default function CategoryItemList({ categories, ...rest }: Props) {
  if (!categories?.length) {
    return <div className={styles.empty}>No categories in this group yet.</div>;
  }

  return (
    <div role="rowgroup">
      {categories.map((category) => (
        <CategoryItem key={category.id} category={category} {...rest} />
      ))}
    </div>
  );
}
