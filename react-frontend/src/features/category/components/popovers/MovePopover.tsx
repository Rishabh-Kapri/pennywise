import { Popover } from '@/components/common/Popover/Popover';
import type { Category, CategoryGroup } from '../../types/category.types';
import { useAppSelector } from '@/app/hooks';
import { selectCategoryGroups } from '../../store';
import styles from './Popover.module.css';
import { AmountCell } from '../AmountCell';
import { selectSelectedMonth } from '@/features/budget';
import { useEffect, useRef, useState } from 'react';

interface Props {
  triggerRef: React.RefObject<HTMLDivElement | null>;
  isOpen: boolean;
  categoryId: string;
  categoryName: string;
  amount: number;
  onClose: () => void;
}

const transformGroups = (groups: CategoryGroup[]) => {
  return groups.filter((group) => !group.isSystem && group.name !== 'Hidden');
};

export function MovePopover({
  triggerRef,
  isOpen,
  categoryId,
  categoryName,
  amount,
  onClose,
}: Props) {
  const [isLocalOpen, setIsLocalOpen] = useState(false);
  const [moveTo, setMoveTo] = useState<Category | null>(null);
  const selectedMonth = useAppSelector(selectSelectedMonth);
  const { allCategoryGroups } = useAppSelector(selectCategoryGroups);
  const transformedGroups = transformGroups(allCategoryGroups);
  const localTriggerRef = useRef<HTMLInputElement | null>(null);

  const handleMoveToSelect = (category: Category | null) => {
    setMoveTo(category);
    setIsLocalOpen(false);
  };

  useEffect(() => {
    if (!isOpen) {
      console.log('Cleaning up MovePopover on close');
      setMoveTo(null);
      setIsLocalOpen(false);
    }
  }, [isOpen]);

  return (
    <Popover
      id={`popover-content-${categoryId}`}
      isOpen={isOpen}
      width={250}
      triggerRef={triggerRef}
      onClose={onClose}>
      <div className={styles.moveContainer}>
        <div className={styles.moveInput}>
          <label htmlFor="moveFrom">Move From "{categoryName}"</label>
          <input
            id="moveFrom"
            type="number"
            className={styles.input}
            value={amount}
            onChange={(e) => console.log('onChange:', e.target.value)}
          />
        </div>
        <div className={styles.moveInput}>
          <label htmlFor="moveTo">To</label>
          <input
            ref={localTriggerRef}
            onClick={() => setIsLocalOpen(true)}
            onFocus={() => setIsLocalOpen(true)}
            onBlur={() => setIsLocalOpen(false)}
            id="moveTo"
            type="text"
            className={styles.input}
            defaultValue={moveTo?.name ?? ''}
            readOnly
          />
          <Popover
            triggerRef={localTriggerRef}
            id={`popover-content-moveTo-${categoryId}`}
            isOpen={isLocalOpen}
            zIndex={1001}>
            <div>
              {transformedGroups.map((group) => (
                <div
                  key={group.id}
                  role="option"
                  tabIndex={0}
                  className={styles.groupContainer}>
                  <div className={styles.title}>{group.name}</div>
                  {group.categories.map((category) => (
                    <div
                      key={category.id}
                      role="option"
                      tabIndex={0}
                      className={`${styles.category} ${categoryId === category?.id ? styles.disabled : ''}`}
                      onMouseDown={(e) => {
                        e.preventDefault();
                        handleMoveToSelect(category);
                      }}>
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
      </div>
    </Popover>
  );
}
