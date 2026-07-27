import { useCallback, useEffect, useRef, useState } from 'react';
import type { Category } from '../types/category.types';
import styles from './CategoryItemList.module.css';
import { MovePopover } from './popovers/MovePopover';
import { AmountCell } from './AmountCell';
import { ActivityPopover } from './ActivityModal';
import { useAppDispatch } from '@/app/hooks';
import { toast } from '@/utils';
import { updateCategoryBudget } from '../store/categorySlice';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import { Parser } from 'expr-eval';

interface Props {
  month: string;
  categories: Category[];
  selectedCategoryId?: string;
  onSelectCategory: (category: Category | null) => void;
  openPopoverId: string | null;
  onPopoverOpen: (id: string) => void;
  onPopoverClose: () => void;
}

interface CategoryItemProps {
  month: string;
  category: Category;
  selectedCategoryId?: string;
  onSelectCategory: (category: Category | null) => void;
  openPopoverId: string | null;
  onPopoverOpen: (id: string) => void;
  onPopoverClose: () => void;
}

const parser = new Parser();

const formatAmount = (value: number): string =>
  getCurrencyLocaleString(Math.abs(value) || 0, 'INR', 'en-IN', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  });

type MeterStatus = 'healthy' | 'warning' | 'danger' | 'idle';

function getMeterStatus(balance: number, percentUsed: number, spent: number): MeterStatus {
  if (balance < 0) return 'danger';
  if (percentUsed >= 80) return 'warning';
  if (spent > 0 || percentUsed > 0) return 'healthy';
  return 'idle';
}

const STATUS_CLASS: Record<MeterStatus, string> = {
  healthy: styles.healthy,
  warning: styles.warning,
  danger: styles.danger,
  idle: styles.idle,
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
  const triggerRef = useRef<HTMLSpanElement | null>(null);
  const activityTriggerRef = useRef<HTMLSpanElement | null>(null);
  const isPopoverOpen = openPopoverId === category.id;
  const [budgeted, setBudgeted] = useState<string>(
    String(category?.budgeted?.[month] ?? 0),
  );
  const [isEditingBudget, setIsEditingBudget] = useState(false);
  const [showActivityModal, setShowActivityModal] = useState(false);

  const dispatch = useAppDispatch();

  const assigned = category?.budgeted?.[month] ?? 0;
  const activity = category?.activity?.[month] ?? 0;
  const balance = category?.balance?.[month] ?? 0;
  const spent = Math.max(-activity, 0);
  const percentUsed = assigned > 0 ? (spent / assigned) * 100 : spent > 0 ? 100 : 0;
  const status = getMeterStatus(balance, percentUsed, spent);
  const fillWidth = Math.min(percentUsed, 100);
  const percentLabel =
    assigned > 0 && (spent > 0 || percentUsed > 0) ? `${Math.round(percentUsed)}%` : '';

  const handleBudgetBlur = useCallback(() => {
    setIsEditingBudget(false);
    const currentBudgeted = category?.budgeted?.[month] ?? 0;
    if (isEditingBudget && category.id) {
      const budgetedNum = Number(budgeted);
      if (budgetedNum !== currentBudgeted) {
        const expr = parser.parse(budgeted);
        const result = expr.evaluate();
        setBudgeted(result.toString());
        dispatch(
          updateCategoryBudget({
            budgeted: result,
            categoryId: category.id,
            month,
          }),
        ).unwrap()
          .then(() => toast.success('Budget updated'))
          .catch(() => toast.error('Failed to update budget'));
      }
    }
  }, [isEditingBudget, budgeted, month, category, dispatch]);

  const onBudgetChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      if (isEditingBudget) {
        if (e.target.value === '') {
          setBudgeted('0');
          return;
        }
        setBudgeted(e.target.value);
      }
    },
    [isEditingBudget],
  );

  const startEditingBudget = useCallback(() => {
    if (!isEditingBudget) {
      setBudgeted(String(category?.budgeted?.[month] ?? 0));
      setIsEditingBudget(true);
    }
  }, [isEditingBudget, category, month]);

  const chipText =
    balance < 0
      ? `${formatAmount(balance)} over`
      : balance > 0
        ? `${formatAmount(balance)} left`
        : formatAmount(0);
  const chipClass = [
    styles.availableChip,
    balance < 0 && styles.chipOver,
    balance > 0 && styles.chipPositive,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div
      onClick={() => onSelectCategory(category)}
      className={`${styles.categoryItem} ${selectedCategoryId === category.id ? styles.selected : ''}`}>
      <div className={styles.topLine}>
        <div className={styles.categoryName}>{category.name}</div>
        <span
          ref={triggerRef}
          id={`${category.id}-balance`}
          className={chipClass}
          onClick={() => onPopoverOpen(category.id ?? '')}
          aria-haspopup={true}
          aria-controls={`popover-content-${category.id}`}>
          {chipText}
        </span>
        <MovePopover
          triggerRef={triggerRef}
          isOpen={isPopoverOpen}
          categoryId={category.id ?? ''}
          categoryName={category.name}
          amount={balance}
          onClose={onPopoverClose}
        />
      </div>

      <div className={`${styles.meterRow} ${STATUS_CLASS[status]}`}>
        <div className={styles.meterTrack}>
          <div className={styles.meterFill} style={{ width: `${fillWidth}%` }} />
        </div>
        {percentLabel && <span className={styles.percentLabel}>{percentLabel}</span>}
      </div>

      <div className={styles.bottomLine}>
        <span
          ref={activityTriggerRef}
          className={styles.activityText}
          onClick={(e) => {
            e.stopPropagation();
            setShowActivityModal(true);
          }}>
          {formatAmount(spent)} of {formatAmount(assigned)} spent
        </span>
        <ActivityPopover
          isOpen={showActivityModal}
          onClose={() => setShowActivityModal(false)}
          triggerRef={activityTriggerRef}
          categoryId={category.id ?? ''}
          categoryName={category.name}
          month={month}
          activityAmount={activity}
        />
        <span className={styles.assignControl}>
          <span className={styles.assignLabel}>Assigned</span>
          <AmountCell
            value={isEditingBudget ? budgeted : assigned}
            isEditing={isEditingBudget}
            onClick={startEditingBudget}
            onBlur={handleBudgetBlur}
            onChange={onBudgetChange}
          />
        </span>
      </div>
    </div>
  );
}

export default function CategoryItemList({
  month,
  categories,
  selectedCategoryId,
  onSelectCategory,
  openPopoverId,
  onPopoverOpen,
  onPopoverClose,
}: Props) {
  useEffect(() => {
    if (!selectedCategoryId) {
      return;
    }
    const handleEscapeKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onSelectCategory(null);
      }
    };

    document.addEventListener('keydown', handleEscapeKey);

    return () => {
      document.removeEventListener('keydown', handleEscapeKey);
    };
  }, [selectedCategoryId, onSelectCategory]);

  return (
    <div className={styles.list}>
      {categories?.length > 0 &&
        categories.map((category) => (
          <CategoryItem
            key={category.id}
            month={month}
            category={category}
            selectedCategoryId={selectedCategoryId}
            onSelectCategory={onSelectCategory}
            openPopoverId={openPopoverId}
            onPopoverOpen={onPopoverOpen}
            onPopoverClose={onPopoverClose}
          />
        ))}
    </div>
  );
}
