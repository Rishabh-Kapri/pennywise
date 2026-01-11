import { ChevronDown, ChevronRight } from 'lucide-react';
import styles from './CategoryGroup.module.css';
import CategoryItemList from './CategoryItemList';
import type { Category, CategoryGroup } from '../types/category.types';
import { CategoryFormDropdown } from './CategoryFormDropdown';
import { useCallback, useState } from 'react';
import { CategoryInfo } from './CategoryInfo';
import { useAppDispatch } from '@/app/hooks';
import { toggleGroupCollapse } from '../store';
import { AmountCell } from './AmountCell';

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

  const handleAddCategory = (category: Category) => {
    setOpenDropdownId(null);
  };

  const handleGroupClose = (group: CategoryGroup) => {
    if (group.id) {
      dispatch(toggleGroupCollapse(group.id));
    }
  };

  return (
    <div className={styles.mainContainer}>
      <div className={styles.header}>
        <div className={styles.headerContainer}>
          <div className={styles.spacer}></div>
          <div className={styles.headerItem}>ASSIGNED</div>
          <div className={styles.headerItem}>ACTIVITY</div>
          <div className={styles.headerItem}>AVAILABLE</div>
        </div>
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
                <div className={styles.groupBudget}>
                  <AmountCell value={group.budgeted?.[month] ?? 0} />
                </div>
                <div className={styles.groupBudget}>
                  <AmountCell value={group.activity?.[month] ?? 0} />
                </div>
                <div className={styles.groupBudget}>
                  <AmountCell
                    value={group.balance?.[month] ?? 0}
                    variant="balance"
                    balanceClassName={styles.groupBalance}
                  />
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
