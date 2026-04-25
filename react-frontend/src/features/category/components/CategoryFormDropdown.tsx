import type React from 'react';
import type { Category } from '../types/category.types';
import { useEffect, useRef, useState } from 'react';
import { useAppSelector } from '@/app/hooks';
import { selectSelectedBudget } from '@/features/budget';
import { Edit, Plus } from 'lucide-react';
import styles from './CategoryFormDropdown.module.css';
import { Dropdown } from '@/components/common/Dropdown/Dropdown';

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
      <Dropdown
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
        triggerRef={triggerRef}
        position="bottom"
        width={340}>
        <form onSubmit={handleSubmit} className={styles.form}>
          <h3 className={styles.title}>
            {category ? 'Edit Category' : 'Add Category'}
          </h3>

          <div className={styles.field}>
            <label htmlFor="name" className={styles.label}>
              Category Name
            </label>
            <input
              id="name"
              name="name"
              type="text"
              value={formData.name}
              onChange={handleChange}
              placeholder="e.g. Groceries"
              className={styles.input}
              required
              autoFocus
            />
          </div>

          <div className={styles.actions}>
            <button type="submit" className={styles.saveButton}>
              {category ? 'Update' : 'Create'}
            </button>
          </div>
        </form>
      </Dropdown>
    </>
  );
}
