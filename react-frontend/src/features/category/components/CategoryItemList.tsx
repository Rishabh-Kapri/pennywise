import { useEffect, useRef, useState } from 'react';
import type { Category } from '../types/category.types';
import styles from './CategoryItemList.module.css';
import { MovePopover } from './popovers/MovePopover';
import { AmountCell } from './AmountCell';

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
  selectedCategoryIdx: number;
  index: number;
  onSelectCategory: (category: Category | null) => void;
  openPopoverId: string | null;
  onPopoverOpen: (id: string) => void;
  onPopoverClose: () => void;
}

export function CategoryItem({
  month,
  category,
  selectedCategoryId,
  selectedCategoryIdx,
  index,
  onSelectCategory,
  openPopoverId,
  onPopoverOpen,
  onPopoverClose,
}: CategoryItemProps) {
  const triggerRef = useRef<HTMLDivElement | null>(null);
  const isPopoverOpen = openPopoverId === category.id;

  return (
    <>
      <div
        key={category.id}
        onClick={() => onSelectCategory(category)}
        className={`${styles.categoryItem} ${selectedCategoryId === category.id ? styles.selected : ''}`}>
        <div className={styles.categoryName}>
          <div>{category.name}</div>
        </div>
        {/* Budgeted */}
        <div className={styles.amountItem}>
          <AmountCell value={category?.budgeted?.[month] ?? 0} />
        </div>
        {/* Activity */}
        <div className={styles.amountItem}>
          <AmountCell value={category?.activity?.[month] ?? 0} />
        </div>
        {/* Balance */}
        <div className={styles.amountItem}>
          <AmountCell
            ref={triggerRef}
            id={`${category.id}-balance`}
            value={category?.balance?.[month] ?? 0}
            variant="balance"
            onClick={() => onPopoverOpen(category.id ?? '')}
            aria-haspopup={true}
            aria-controls={`popover-content-${category.id}`}
          />
          <MovePopover
            triggerRef={triggerRef}
            isOpen={isPopoverOpen}
            categoryId={category.id ?? ''}
            categoryName={category.name}
            amount={category.balance?.[month] ?? 0}
            onClose={onPopoverClose}
          />
        </div>
      </div>
      <hr
        className={`${selectedCategoryIdx !== -1 &&
            (selectedCategoryIdx === index || selectedCategoryIdx - 1 === index)
            ? styles.borderNone
            : styles.categoryDivider
          }`}
      />
    </>
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
  const [selectedCategoryIdx, setSelectedCategoryIdx] = useState<number>(-1);

  useEffect(() => {
    if (!selectedCategoryId) {
      setSelectedCategoryIdx(-1);
      return;
    }
    const handleEscapeKey = (event: KeyboardEvent) => {
      console.log('handling esacape:', event.key);
      if (event.key === 'Escape') {
        setSelectedCategoryIdx(-1);
        onSelectCategory(null);
      }
    };
    const idx = categories.map((c) => c.id).indexOf(selectedCategoryId);
    setSelectedCategoryIdx(idx);

    document.addEventListener('keydown', handleEscapeKey);

    return () => {
      document.removeEventListener('keydown', handleEscapeKey);
    };
  }, [selectedCategoryId, categories, onSelectCategory]);

  return (
    <>
      {categories?.length === 0 && <div></div>}
      {categories?.length > 0 &&
        categories.map((category, idx) => (
          <CategoryItem
            key={category.id}
            month={month}
            category={category}
            selectedCategoryId={selectedCategoryId}
            selectedCategoryIdx={selectedCategoryIdx}
            index={idx}
            onSelectCategory={onSelectCategory}
            openPopoverId={openPopoverId}
            onPopoverOpen={onPopoverOpen}
            onPopoverClose={onPopoverClose}
          />
        ))}
    </>
  );
}
