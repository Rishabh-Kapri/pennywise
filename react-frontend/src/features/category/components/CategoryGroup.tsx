import { CaretDown as ChevronDown, CaretRight as ChevronRight } from '@phosphor-icons/react';
import styles from './CategoryGroup.module.css';
import CategoryItemList from './CategoryItemList';
import type { Category, CategoryGroup } from '../types/category.types';
import { CategoryFormDropdown } from './CategoryFormDropdown';
import { useCallback, useState } from 'react';
import { CategoryInfo } from './CategoryInfo';
import { useAppDispatch } from '@/app/hooks';
import { createCategory, toggleGroupCollapse } from '../store';
import { toast } from '@/utils';
import { getCurrencyLocaleString } from '@/utils/date.utils';

const formatAmount = (value: number): string =>
  getCurrencyLocaleString(value || 0, 'INR', 'en-IN', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  });

interface CategoryGroupProps {
  groups: CategoryGroup[];
  month: string;
}

export default function CategoryGroup({ groups, month }: CategoryGroupProps) {
  const [openDropdownId, setOpenDropdownId] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(
    null,
  );
  const [openPopoverId, setOpenPopoverId] = useState<string | null>(null);
  const dispatch = useAppDispatch();

  const handlePopoverOpen = useCallback((id: string) => {
    setOpenPopoverId(id);
  }, []);

  const handlePopoverClose = useCallback(() => {
    setOpenPopoverId(null);
  }, []);

  const handleCategorySelect = useCallback((category: Category | null) => {
    setSelectedCategory(category);
    if (!category) {
      setOpenPopoverId(null);
    }
  }, []);

  const handleAddCategory = async (category: Category) => {
    try {
      await dispatch(
        createCategory({
          name: category.name.trim(),
          budgetId: category.budgetId,
          categoryGroupId: category.categoryGroupId,
          budgeted: category.budgeted,
        }),
      ).unwrap();
      setOpenDropdownId(null);
      toast.success('Category created');
    } catch (error) {
      const message =
        error instanceof Error ? error.message : 'Failed to create category';
      toast.error(message);
      throw error;
    }
  };

  const handleGroupClose = (group: CategoryGroup) => {
    if (group.id) {
      dispatch(toggleGroupCollapse(group.id));
    }
  };

  return (
    <div className={styles.mainContainer}>
      <div className={styles.header}>
        <div className={styles.content}>
          {groups.map((group) => (
            <div key={group.id} className={styles.groupContainer}>
              <div className={styles.groupHeader}>
                <div className={styles.groupInfo}>
                  {group.collapsed && (
                    <ChevronRight
                      className={styles.icon}
                      onClick={() => handleGroupClose(group)}
                    />
                  )}
                  {!group.collapsed && (
                    <ChevronDown
                      className={styles.icon}
                      onClick={() => handleGroupClose(group)}
                    />
                  )}
                  <div className={styles.groupName}>{group.name}</div>
                  {group.name !== 'Hidden' && (
                    <>
                      <div className={styles.addCategory}>
                        <CategoryFormDropdown
                          groupId={group.id}
                          onSave={handleAddCategory}
                          isOpen={openDropdownId === group.id}
                          onOpenChange={(open) => {
                            return setOpenDropdownId(open ? group.id! : null);
                          }}
                        />
                      </div>
                    </>
                  )}
                </div>
                <div className={styles.groupSummary}>
                  <span className={styles.groupAssigned}>
                    {formatAmount(group.budgeted?.[month] ?? 0)} assigned
                  </span>
                  <span
                    className={`${styles.groupChip} ${
                      (group.balance?.[month] ?? 0) < 0 ? styles.groupChipOver : ''
                    }`}>
                    {formatAmount(Math.abs(group.balance?.[month] ?? 0))}{' '}
                    {(group.balance?.[month] ?? 0) < 0 ? 'over' : 'left'}
                  </span>
                </div>
              </div>
              {!group.collapsed && (
                <CategoryItemList
                  key={group.id}
                  month={month}
                  categories={group.categories}
                  selectedCategoryId={selectedCategory?.id}
                  onSelectCategory={handleCategorySelect}
                  openPopoverId={openPopoverId}
                  onPopoverOpen={handlePopoverOpen}
                  onPopoverClose={handlePopoverClose}
                />
              )}
            </div>
          ))}
        </div>
      </div>
      <CategoryInfo category={selectedCategory} />
    </div>
  );
}
