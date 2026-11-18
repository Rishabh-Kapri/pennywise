import { useEffect, useRef, useState } from 'react';
import { Popover } from '@/components/common/Popover/Popover';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { selectSelectedMonth } from '@/features/budget/store/budgetSlice';
import { toast } from '@/utils';
import { evaluateAmount } from '@/utils/expression';
import type { Category, CategoryGroup } from '../../types/category.types';
import { selectCategoryGroups, updateCategoryBudget } from '@/features/category/store/categorySlice';
import { AmountCell } from '../AmountCell';
import styles from './Popover.module.css';

interface Props {
  triggerRef: React.RefObject<HTMLElement | null>;
  isOpen: boolean;
  categoryId: string;
  categoryName: string;
  /** the category's available balance this month */
  amount: number;
  onClose: () => void;
}

const transformGroups = (groups: CategoryGroup[]) =>
  groups.filter((group) => !group.isSystem && group.name !== 'Hidden');

/**
 * Move money between categories for the selected month. A positive balance
 * moves out of this category; an overspent one is covered from the chosen
 * category. Both sides are changes to what's assigned.
 */
export function MovePopover({ triggerRef, isOpen, categoryId, categoryName, amount, onClose }: Props) {
  const dispatch = useAppDispatch();
  const selectedMonth = useAppSelector(selectSelectedMonth);
  const { allCategoryGroups } = useAppSelector(selectCategoryGroups);
  const groups = transformGroups(allCategoryGroups);
  const covering = amount < 0;

  const [draft, setDraft] = useState('');
  const [other, setOther] = useState<Category | null>(null);
  const [toReady, setToReady] = useState(false);
  const [listOpen, setListOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const pickerRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (isOpen) {
      setDraft(String(Math.abs(amount)));
    } else {
      setOther(null);
      setToReady(false);
      setListOpen(false);
      setBusy(false);
    }
  }, [isOpen, amount]);

  const assignedOf = (id: string): number => {
    for (const g of allCategoryGroups) {
      const c = g.categories.find((cat) => cat.id === id);
      if (c) return c.budgeted?.[selectedMonth] ?? 0;
    }
    return 0;
  };

  const pick = (category: Category | null) => {
    setOther(category);
    setToReady(category === null);
    setListOpen(false);
  };

  const move = async () => {
    let value = evaluateAmount(draft) ?? NaN;
    if (Number.isNaN(value)) {
      toast.error('Enter an amount, or a sum like 1200+300');
      return;
    }
    if (value <= 0) {
      toast.error('Enter an amount more than 0');
      return;
    }
    if (!other && !toReady) {
      toast.error(covering ? 'Choose where to take it from' : 'Choose where to move it');
      return;
    }
    value = Math.round(value * 100) / 100;
    // from loses what to gains; Ready to Assign has no row to change
    const from = covering ? other?.id : categoryId;
    const to = covering ? categoryId : other?.id;
    setBusy(true);
    try {
      if (from) {
        await dispatch(
          updateCategoryBudget({ categoryId: from, month: selectedMonth, budgeted: assignedOf(from) - value }),
        ).unwrap();
      }
      if (to) {
        await dispatch(
          updateCategoryBudget({ categoryId: to, month: selectedMonth, budgeted: assignedOf(to) + value }),
        ).unwrap();
      }
      toast.success('Money moved');
      onClose();
    } catch {
      toast.error("Couldn't move the money", { description: 'Check both categories and try again.' });
      setBusy(false);
    }
  };

  const otherLabel = toReady ? 'Ready to Assign' : (other?.name ?? '');

  return (
    <Popover id={`popover-content-${categoryId}`} isOpen={isOpen} width={280} triggerRef={triggerRef} onClose={onClose}>
      <form
        className={styles.moveContainer}
        onSubmit={(e) => {
          e.preventDefault();
          void move();
        }}
      >
        <div className={styles.moveInput}>
          <label htmlFor={`move-amount-${categoryId}`}>
            {covering ? `Cover "${categoryName}" overspending` : `Move from "${categoryName}"`}
          </label>
          <input
            id={`move-amount-${categoryId}`}
            type="text"
            inputMode="decimal"
            className={styles.input}
            value={draft}
            autoFocus
            onChange={(e) => setDraft(e.target.value)}
          />
        </div>
        <div className={styles.moveInput}>
          <label htmlFor={`move-to-${categoryId}`}>{covering ? 'From' : 'To'}</label>
          <button
            id={`move-to-${categoryId}`}
            ref={pickerRef}
            type="button"
            className={`${styles.input} ${styles.movePicker}`}
            aria-haspopup="listbox"
            aria-expanded={listOpen}
            onClick={() => setListOpen((open) => !open)}
          >
            {otherLabel || <span className={styles.movePlaceholder}>Choose a category</span>}
          </button>
          <Popover
            triggerRef={pickerRef}
            id={`popover-content-moveTo-${categoryId}`}
            isOpen={listOpen}
            onClose={() => setListOpen(false)}
            zIndex={1001}
          >
            <div role="listbox" className={styles.moveList}>
              {!covering && (
                <div
                  role="option"
                  aria-selected={toReady}
                  tabIndex={0}
                  className={styles.category}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    pick(null);
                  }}
                >
                  <span>Ready to Assign</span>
                </div>
              )}
              {groups.map((group) => (
                <div key={group.id} className={styles.groupContainer}>
                  <div className={styles.title}>{group.name}</div>
                  {group.categories.map((category) => (
                    <div
                      key={category.id}
                      role="option"
                      aria-selected={other?.id === category.id}
                      tabIndex={0}
                      className={`${styles.category} ${categoryId === category.id ? styles.disabled : ''}`}
                      onMouseDown={(e) => {
                        e.preventDefault();
                        if (categoryId !== category.id) pick(category);
                      }}
                    >
                      <span>{category.name}</span>
                      <AmountCell
                        value={category.balance?.[selectedMonth] ?? 0}
                        variant="balance"
                        balanceClassName={styles.balance}
                        overspentClassName={styles.overspent}
                      />
                    </div>
                  ))}
                </div>
              ))}
            </div>
          </Popover>
        </div>
        <div className={styles.moveActions}>
          <button type="button" className={styles.moveCancel} onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className={styles.moveSubmit} disabled={busy || (!other && !toReady)}>
            {busy ? 'Moving…' : 'Move'}
          </button>
        </div>
      </form>
    </Popover>
  );
}
