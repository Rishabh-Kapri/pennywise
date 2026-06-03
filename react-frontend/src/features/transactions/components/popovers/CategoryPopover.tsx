import { type KeyboardEvent, useEffect, useMemo, useRef, useState } from 'react';
import { X } from '@phosphor-icons/react';
import styles from './Popover.module.css';
import { useAppSelector } from '@/app/hooks';
import { selectCategoryGroups } from '@/features/category';
import { Popover } from '@/components/common/Popover/Popover';
import type { Category, CategoryGroup } from '@/features/category/types/category.types';
import { selectSelectedMonth } from '@/features/budget';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import { selectInflowCategory } from '@/features/category/store/categorySlice';
import type { TransactionDropdownProps } from './types';

const transformGroups = (groups: CategoryGroup[]) => {
  return groups.filter((group) => !group.isSystem && group.name !== 'Hidden');
};

export function CategoryDropdown({
  value,
  onClick,
  autoFocus,
  variant = 'inline',
  multiple = false,
  selectedIds = [],
  onChangeMultiple,
}: TransactionDropdownProps) {
  const { allCategoryGroups } = useAppSelector(selectCategoryGroups);
  const inflowCategory = useAppSelector(selectInflowCategory);

  const transformedGroups = useMemo(() => transformGroups(allCategoryGroups), [allCategoryGroups]);

  const groupsWithInflow = useMemo(() => {
    if (inflowCategory) {
      const inflowGroup: CategoryGroup = {
        id: 'inflow-group',
        name: 'Inflow',
        isSystem: false,
        collapsed: false,
        balance: {},
        budgeted: {},
        activity: {},
        categories: [inflowCategory],
      };
      return [inflowGroup, ...transformedGroups];
    }
    return transformedGroups;
  }, [inflowCategory, transformedGroups]);

  const selectedMonth = useAppSelector(selectSelectedMonth);

  const [isOpen, setIsOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);

  const filteredItems = useMemo(() => {
    const normalized = searchQuery.trim().toLowerCase();

    if (!normalized) {
      return groupsWithInflow;
    }

    return groupsWithInflow
      .map((group) => {
        const groupMatches = group.name.trim().toLowerCase().includes(normalized);
        const categories = groupMatches
          ? group.categories
          : group.categories.filter((cat) => cat.name.trim().toLowerCase().includes(normalized));

        return {
          ...group,
          categories,
        };
      })
      .filter((group) => group.categories.length > 0);
  }, [groupsWithInflow, searchQuery]);

  useEffect(() => {
    if (autoFocus) {
      setIsOpen(true);
    }
  }, [autoFocus]);

  useEffect(() => {
    if (isOpen) {
      searchRef.current?.focus();
    }
  }, [isOpen]);

  const handleOnClick = (category: Category) => {
    if (multiple) {
      if (!onChangeMultiple) return;
      const catId = category.id!;
      const isSelected = selectedIds.includes(catId);
      let nextIds: string[];
      let nextNames: string[];

      // Gather all categories to find the names later
      const allCategoriesList: Category[] = [];
      for (const g of groupsWithInflow) {
        allCategoriesList.push(...(g.categories ?? []));
      }

      if (isSelected) {
        nextIds = selectedIds.filter((id) => id !== catId);
        nextNames = allCategoriesList
          .filter((cat) => nextIds.includes(cat.id!))
          .map((cat) => cat.name);
      } else {
        nextIds = [...selectedIds, catId];
        nextNames = allCategoriesList
          .filter((cat) => nextIds.includes(cat.id!))
          .map((cat) => cat.name);
      }
      onChangeMultiple(nextIds, nextNames);
    } else {
      setIsOpen(false);
      setSearchQuery('');
      onClick(category.id!, category.name);
    }
  };

  const handleClearCategory = () => {
    if (multiple) {
      if (onChangeMultiple) {
        onChangeMultiple([], []);
      }
    } else {
      setIsOpen(false);
      setSearchQuery('');
      onClick('', '');
    }
  };

  const handleSearchKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter') return;

    const firstCategory = filteredItems.find((group) => group.categories.length > 0)?.categories[0];
    if (!firstCategory) return;

    e.preventDefault();
    handleOnClick(firstCategory);
  };

  const triggerClassName = variant === 'form' ? styles.formTrigger : styles.categoryTrigger;
  const displayValue = value || 'Select Category';

  return (
    <div className={styles.popoverContainer}>
      <button
        type="button"
        ref={triggerRef}
        onClick={() => setIsOpen((prev) => !prev)}
        className={`${triggerClassName} ${styles.triggerButton} ${isOpen ? styles.open : ''}`}
        autoFocus={autoFocus}
        aria-haspopup="true"
        aria-expanded={isOpen}
        aria-controls="category-popover-content"
      >
        {displayValue}
      </button>
      <Popover
        id="category-popover-content"
        isOpen={isOpen}
        triggerRef={triggerRef}
        onClose={() => setIsOpen(false)}
        width={400}
      >
        <div className={styles.searchContainer}>
          <input
            ref={searchRef}
            className={styles.searchInput}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onKeyDown={handleSearchKeyDown}
            placeholder="Search categories"
            aria-label="Search categories"
          />
        </div>
        {value && (
          <button
            type="button"
            className={styles.clearSelectionItem}
            onMouseDown={(e) => {
              e.preventDefault();
              handleClearCategory();
            }}
          >
            <X size={16} />
            <span>Remove selected category</span>
          </button>
        )}
        {filteredItems.length > 0 ? (
          filteredItems.map((group) => (
            <div key={group.id} role="option" className={styles.groupContainer}>
              <div className={styles.title}>{group.name}</div>
              {group.categories.map((category) => {
                const isSelected = multiple
                  ? selectedIds.includes(category.id!)
                  : category.name === value;
                return (
                  <div
                    key={category.id}
                    tabIndex={0}
                    className={`${styles.item} ${styles.category} ${isSelected ? styles.selectedItem : ''}`}
                    role="option"
                    aria-selected={isSelected}
                    onMouseDown={(e) => {
                      e.preventDefault();
                      handleOnClick(category);
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        handleOnClick(category);
                      }
                    }}
                  >
                    <div>{category.name}</div>
                    <div
                      className={`
                      ${styles.amount} 
                      ${(category.balance?.[selectedMonth] ?? 0) === 0
                          ? ''
                          : (category.balance?.[selectedMonth] ?? 0) > 0
                            ? styles.balance
                            : styles.overspent
                        }`}
                    >
                      {getCurrencyLocaleString(category.balance?.[selectedMonth] ?? 0)}
                    </div>
                  </div>
                );
              })}
            </div>
          ))
        ) : (
          <div className={styles.emptyState}>No categories found</div>
        )}
      </Popover>
    </div>
  );
}
