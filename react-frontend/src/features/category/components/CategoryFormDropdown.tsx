import type React from 'react';
import type { Category } from '../types/category.types';
import { useRef, useState } from 'react';
import { useAppSelector } from '@/app/hooks';
import { selectSelectedBudget } from '@/features/budget';
import { Edit, Plus } from 'lucide-react';
import styles from './CategoryFormDropdown.module.css';
import { Dropdown } from '@/components/common/Dropdown/Dropdown';

interface CategoryFormDropdownProps {
  category?: Category;
  groupId?: string;
  onSave: (category: Category) => void;
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
    category || {
      name: '',
      categoryGroupId: groupId ?? '',
      budgetId: selectedBudget?.id ?? '',
      budgeted: {},
    },
  );
  const triggerRef = useRef<HTMLDivElement>(null);

  const isOpen = parentIsOpen !== undefined ? parentIsOpen : internalIsOpen;
  const setIsOpen = (open: boolean) => {
    console.trace('setIsOpen:', open);
    if (onOpenChange) {
      onOpenChange(open);
    } else {
      setIsInternalIsOpen(open);
    }
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    onSave(formData);
    setIsOpen(false);
    if (!category) {
      setFormData({
        name: '',
        categoryGroupId: '',
        budgetId: selectedBudget?.id ?? '',
        budgeted: {},
      });
    }
  };

  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = event.target;
    console.log('handleChange:', name, value);
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
