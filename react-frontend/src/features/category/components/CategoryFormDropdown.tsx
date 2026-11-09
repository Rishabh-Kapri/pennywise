import type React from 'react';
import type { Category } from '../types/category.types';
import { useEffect, useRef, useState } from 'react';
import { useAppSelector } from '@/app/hooks';
import { selectSelectedBudget } from '@/features/budget';
import { ArrowRight, PencilSimple as Edit, Plus } from '@phosphor-icons/react';
import styles from './CategoryFormDropdown.module.css';
import { Popover } from '@/components/common/Popover/Popover';

interface CategoryFormDropdownProps {
  category?: Category;
  groupId?: string;
  onSave: (category: Category) => Promise<void> | void;
  isOpen?: boolean; // allows the parent to hide the dropdown
  onOpenChange?: (open: boolean) => void;
  trigger?: React.ReactNode;
}
export function CategoryFormDropdown({
  category,
  groupId,
  onSave,
  isOpen: parentIsOpen,
  onOpenChange,
  trigger,
}: CategoryFormDropdownProps) {
  const [internalIsOpen, setIsInternalIsOpen] = useState(false);
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const [formData, setFormData] = useState<Category>(
    category ?? {
      name: '',
      categoryGroupId: groupId ?? '',
      budgetId: selectedBudget?.id ?? '',
      budgeted: {},
    },
  );
  const triggerRef = useRef<HTMLDivElement>(null);

  const isOpen = parentIsOpen !== undefined ? parentIsOpen : internalIsOpen;

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    setFormData(
      category ?? {
        name: '',
        categoryGroupId: groupId ?? '',
        budgetId: selectedBudget?.id ?? '',
        budgeted: {},
      },
    );
  }, [category, groupId, isOpen, selectedBudget?.id]);

  const setIsOpen = (open: boolean) => {
    if (onOpenChange) {
      onOpenChange(open);
    } else {
      setIsInternalIsOpen(open);
    }
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    const payload: Category = {
      ...formData,
      name: formData.name.trim(),
      categoryGroupId: formData.categoryGroupId || groupId || '',
      budgetId: formData.budgetId || selectedBudget?.id || '',
      budgeted: formData.budgeted ?? {},
    };

    try {
      await onSave(payload);
      setIsOpen(false);
      if (!category) {
        setFormData({
          name: '',
          categoryGroupId: groupId ?? '',
          budgetId: selectedBudget?.id ?? '',
          budgeted: {},
        });
      }
    } catch {
      return;
    }
  };

  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = event.target;
    setFormData((prev) => ({
      ...prev,
      [name]: value,
    }));
  };

  return (
    <>
      <div
        ref={triggerRef}
        onClick={() => setIsOpen(!isOpen)}
        className={isOpen ? styles.dropdownOpen : ''}>
        {trigger || (
          <div className={styles.defaultTrigger}>
            {category ? <Edit size={16} /> : <Plus size={16} />}
            <span>{category ? 'Edit' : 'Add Category'}</span>
          </div>
        )}
      </div>
      <Popover
        id={`category-form-${category?.id ?? groupId ?? 'new'}`}
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
        triggerRef={triggerRef}
        placement="bottom"
        width={300}>
        <form onSubmit={handleSubmit} className={styles.form}>
          <label htmlFor="category-form-name" className={styles.label}>
            {category ? 'Rename category' : 'New category'}
          </label>
          <div className={styles.inputRow}>
            <input
              id="category-form-name"
              name="name"
              type="text"
              value={formData.name}
              onChange={handleChange}
              placeholder="e.g. Groceries"
              className={styles.input}
              autoComplete="off"
              required
              autoFocus
            />
            <button
              type="submit"
              className={styles.saveButton}
              disabled={!formData.name.trim()}
              aria-label={category ? 'Save category' : 'Create category'}>
              <ArrowRight size={16} weight="bold" />
            </button>
          </div>
          <span className={styles.hint}>Press Enter to {category ? 'save' : 'add'} · Esc to cancel</span>
        </form>
      </Popover>
    </>
  );
}
