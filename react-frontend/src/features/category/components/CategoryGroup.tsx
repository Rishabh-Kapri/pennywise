import { CaretRight, Plus } from '@phosphor-icons/react';
import styles from './CategoryGroup.module.css';
import CategoryItemList from './CategoryItemList';
import type { Category, CategoryGroup } from '../types/category.types';
import { CategoryFormDropdown } from './CategoryFormDropdown';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { CategoryInfo } from './CategoryInfo';
import { MonthSummary } from './MonthSummary';
import { GroupRenameButton } from './CategoryActions';
import { useAppDispatch } from '@/app/hooks';
import { createCategory, HIDDEN_GROUP_ID, toggleGroupCollapse } from '../store';
import { toast } from '@/utils';
import { formatBudgetAmount } from './budgetFormat';

interface CategoryGroupProps {
  groups: CategoryGroup[];
  month: string;
}

export default function CategoryGroup({ groups, month }: CategoryGroupProps) {
  const [openDropdownId, setOpenDropdownId] = useState<string | null>(null);
  // Keep the id, not the object: the category is re-read from the store so the
  // inspector reflects edits (assigned amounts, moves) as soon as they land.
  const [selectedCategoryId, setSelectedCategoryId] = useState<string | null>(null);
  const [openPopoverId, setOpenPopoverId] = useState<string | null>(null);
  const dispatch = useAppDispatch();

  const selected = useMemo(() => {
    if (!selectedCategoryId) return null;
    for (const group of groups) {
      const match = group.categories?.find((c) => c.id === selectedCategoryId);
      if (match) return { category: match, groupName: group.name };
    }
    return null;
  }, [groups, selectedCategoryId]);
  const selectedCategory = selected?.category ?? null;

  const handlePopoverClose = useCallback(() => setOpenPopoverId(null), []);
  const handleCategorySelect = useCallback((category: Category) => {
    setSelectedCategoryId((current) => (current === category.id ? null : category.id ?? null));
  }, []);
  const clearSelection = useCallback(() => {
    setSelectedCategoryId(null);
    setOpenPopoverId(null);
  }, []);

  useEffect(() => {
    if (!selectedCategoryId) return;
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') clearSelection();
    };
    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [selectedCategoryId, clearSelection]);

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
      const message = error instanceof Error ? error.message : 'Failed to create category';
      toast.error(message);
      throw error;
    }
  };

  return (
    <div className={styles.layout}>
      <div className={styles.table} role="table" aria-label="Budget categories">
        <div className={styles.columns} role="row">
          <span role="columnheader">Category</span>
          <span role="columnheader" className={styles.numericHeader}>Assigned</span>
          <span role="columnheader" className={styles.numericHeader}>Activity</span>
          <span role="columnheader" className={styles.numericHeader}>Available</span>
        </div>

        <div className={styles.body}>
          {groups.map((group) => {
            const balance = group.balance?.[month] ?? 0;
            const canAdd = group.name !== 'Hidden' && !group.isSystem;
            return (
              <section
                key={group.id}
                className={`${styles.group} ${group.id === HIDDEN_GROUP_ID ? styles.hiddenGroup : ''}`}>
                <div className={styles.groupRow} role="row">
                  <div className={styles.groupName} role="rowheader">
                    <button
                      type="button"
                      className={styles.toggle}
                      aria-expanded={!group.collapsed}
                      onClick={() => group.id && dispatch(toggleGroupCollapse(group.id))}>
                      <CaretRight
                        size={12}
                        weight="bold"
                        className={`${styles.caret} ${group.collapsed ? '' : styles.caretOpen}`}
                      />
                      <span className={styles.groupLabel}>{group.name}</span>
                    </button>
                    <span className={styles.groupActions}>
                      <GroupRenameButton group={group} />
                    </span>
                    {canAdd && (
                      <div
                        className={`${styles.addCategory} ${openDropdownId === group.id ? styles.addOpen : ''}`}>
                        <CategoryFormDropdown
                          groupId={group.id}
                          onSave={handleAddCategory}
                          isOpen={openDropdownId === group.id}
                          onOpenChange={(open) => setOpenDropdownId(open ? group.id! : null)}
                          trigger={
                            <span
                              className={styles.addButton}
                              role="button"
                              tabIndex={0}
                              aria-label={`Add category to ${group.name}`}
                              title="Add category">
                              <Plus size={13} weight="bold" />
                            </span>
                          }
                        />
                      </div>
                    )}
                  </div>
                  <span className={`${styles.groupNum} ${styles.desktopOnly}`} role="cell">
                    {formatBudgetAmount(group.budgeted?.[month] ?? 0)}
                  </span>
                  <span className={`${styles.groupNum} ${styles.desktopOnly}`} role="cell">
                    {formatBudgetAmount(group.activity?.[month] ?? 0)}
                  </span>
                  <span
                    className={`${styles.groupNum} ${balance < 0 ? styles.negative : ''}`}
                    role="cell">
                    {formatBudgetAmount(balance)}
                  </span>
                </div>
                {!group.collapsed && (
                  <CategoryItemList
                    month={month}
                    categories={group.categories}
                    selectedCategoryId={selectedCategoryId ?? undefined}
                    onSelectCategory={handleCategorySelect}
                    openPopoverId={openPopoverId}
                    onPopoverOpen={setOpenPopoverId}
                    onPopoverClose={handlePopoverClose}
                  />
                )}
              </section>
            );
          })}
        </div>
      </div>

      {selectedCategory ? (
        <>
          <button
            type="button"
            className={styles.backdrop}
            aria-label="Close category details"
            onClick={clearSelection}
          />
          <CategoryInfo
            key={selectedCategory.id}
            category={selectedCategory}
            groupName={selected?.groupName}
            onClose={clearSelection}
          />
        </>
      ) : (
        <MonthSummary groups={groups} month={month} onSelectCategory={handleCategorySelect} />
      )}
    </div>
  );
}
