import { useMemo, useRef } from 'react';
import styles from './Popover.module.css';
import { useAppSelector } from '@/app/hooks';
import { selectCategoryGroups } from '@/features/category';
import { Popover } from '@/components/common/Popover/Popover';
import type {
  Category,
  CategoryGroup,
} from '@/features/category/types/category.types';
import { selectSelectedMonth } from '@/features/budget';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import { useDropdown } from '../../hooks/useDropdown';
import { selectInflowCategory } from '@/features/category/store/categorySlice';

interface Props {
  value: string;
  onClick: (id: string, name: string) => void;
}

const transformGroups = (groups: CategoryGroup[]) => {
  return groups.filter((group) => !group.isSystem && group.name !== 'Hidden');
};

export function CategoryDropdown({ value, onClick }: Props) {
  const { allCategoryGroups } = useAppSelector(selectCategoryGroups);
  const inflowCategory = useAppSelector(selectInflowCategory);

  const transformedGroups = transformGroups(allCategoryGroups);

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
        categories: [inflowCategory], // <--- The inflow category itself
      };
      return [inflowGroup, ...transformedGroups];
    }
    return transformedGroups;
  }, [inflowCategory, transformedGroups]);

  const selectedMonth = useAppSelector(selectSelectedMonth);

  const filterFn = (groups: CategoryGroup[], filterQuery: string) => {
    return groups.filter(
      (group) =>
        group.name.trim().toLowerCase().includes(filterQuery) ||
        group.categories.filter((cat) =>
          cat.name.trim().toLowerCase().includes(filterQuery),
        ).length > 0,
    );
  };

  const {
    isOpen,
    setIsOpen,
    filterQuery,
    setFilterQuery,
    filteredItems,
    filterValues,
  } = useDropdown(value, groupsWithInflow, filterFn);

  const triggerRef = useRef<HTMLInputElement | null>(null);

  const handleOnBlur = () => {
    setIsOpen(false);
  };

  const handleOnClick = (category: Category) => {
    console.log('handleOnClick:', category);
    setIsOpen(false);
    setFilterQuery(category.name);
    onClick(category.id!, category.name);
  };

  return (
    <div className={styles.popoverContainer}>
      <input
        ref={triggerRef}
        onFocus={() => setIsOpen(true)}
        onBlur={handleOnBlur}
        className={`${styles.input} ${styles.trigger}`}
        onChange={(e) => filterValues(e.target.value)}
        value={filterQuery}
        placeholder="Select Category"
        aria-haspopup="true"
        aria-expanded={isOpen}
        aria-controls="popover-content"
      />
      {filteredItems.length > 0 && (
        <Popover id={'popover-content'} isOpen={isOpen} triggerRef={triggerRef}>
          {filteredItems.map((group) => (
            <div key={group.id} role="option" className={styles.groupContainer}>
              <div className={styles.title}>{group.name}</div>
              {group.categories.map((category) => (
                <div
                  key={category.id}
                  tabIndex={0}
                  className={`${styles.item} ${styles.category}`}
                  role="option"
                  onMouseDown={(e) => {
                    e.preventDefault();
                    handleOnClick(category);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      handleOnClick(category);
                    }
                  }}>
                  <div>{category.name}</div>
                  <div
                    className={`
                    ${styles.amount} 
                    ${(category.balance?.[selectedMonth] ?? 0) === 0
                        ? ''
                        : (category.balance?.[selectedMonth] ?? 0) > 0
                          ? styles.balance
                          : styles.overspent
                      }`}>
                    {getCurrencyLocaleString(
                      category.balance?.[selectedMonth] ?? 0,
                    )}
                  </div>
                </div>
              ))}
            </div>
          ))}
        </Popover>
      )}
    </div>
  );
}
