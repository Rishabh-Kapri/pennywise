import type React from 'react';
import { useRef, useState } from 'react';
import {
  ArrowLeft,
  ArrowRight,
  CaretRight,
  Check,
  DotsThree,
  Eye,
  EyeSlash,
  FolderSimple,
  PencilSimple,
} from '@phosphor-icons/react';
import { Popover } from '@/components/common/Popover/Popover';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { errorMessage, toast } from '@/utils';
import type { Category, CategoryGroup } from '../types/category.types';
import {
  HIDDEN_GROUP_ID,
  renameCategoryGroup,
  selectCategoryGroups,
  updateCategory,
  type CategoryChanges,
} from '../store/categorySlice';
import styles from './CategoryActions.module.css';

interface RenameFormProps {
  label: string;
  initial: string;
  onSave: (name: string) => Promise<void>;
}

function RenameForm({ label, initial, onSave }: RenameFormProps) {
  const [name, setName] = useState(initial);
  const [saving, setSaving] = useState(false);
  const trimmed = name.trim();

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!trimmed || saving) return;
    setSaving(true);
    try {
      await onSave(trimmed);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form className={styles.form} onSubmit={submit}>
      <label className={styles.formLabel} htmlFor="rename-input">
        {label}
      </label>
      <div className={styles.inputRow}>
        <input
          id="rename-input"
          className={styles.input}
          value={name}
          onChange={(e) => setName(e.target.value)}
          onFocus={(e) => e.target.select()}
          maxLength={100}
          autoComplete="off"
          autoFocus
        />
        <button
          type="submit"
          className={styles.saveButton}
          disabled={!trimmed || trimmed === initial || saving}
          aria-label="Save name">
          <ArrowRight size={16} weight="bold" />
        </button>
      </div>
      <span className={styles.hint}>Enter to save · Esc to cancel</span>
    </form>
  );
}

type Mode = 'menu' | 'rename' | 'move';

interface CategoryActionsProps {
  category: Category;
}

/** Rename, move to another group, hide/unhide. */
export function CategoryActions({ category }: CategoryActionsProps) {
  const dispatch = useAppDispatch();
  const { allCategoryGroups } = useAppSelector(selectCategoryGroups);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [mode, setMode] = useState<Mode>('menu');

  if (category.isSystem) return null;

  const close = () => {
    setIsOpen(false);
    setMode('menu');
  };

  const apply = async (changes: CategoryChanges, success: string) => {
    try {
      await dispatch(updateCategory({ category, changes })).unwrap();
      toast.success(success);
      close();
    } catch (error) {
      toast.error("Couldn't update category", { description: errorMessage(error, 'Please try again.') });
    }
  };

  const destinations = allCategoryGroups.filter((g) => !g.isSystem && g.id !== HIDDEN_GROUP_ID);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={`${styles.trigger} ${isOpen ? styles.triggerOpen : ''}`}
        aria-label={`${category.name} options`}
        aria-haspopup="menu"
        aria-expanded={isOpen}
        onClick={(e) => {
          e.stopPropagation();
          if (isOpen) close();
          else setIsOpen(true);
        }}>
        <DotsThree size={18} weight="bold" />
      </button>
      <Popover
        id={`category-actions-${category.id}`}
        isOpen={isOpen}
        onClose={close}
        triggerRef={triggerRef}
        placement="bottom"
        width={260}>
        {/* the popover is portalled, but React still bubbles its clicks to the row */}
        <div onClick={(e) => e.stopPropagation()}>
          {mode === 'menu' && (
            <div className={styles.menu} role="menu" aria-label={`${category.name} options`}>
              <button type="button" role="menuitem" className={styles.item} onClick={() => setMode('rename')}>
                <PencilSimple size={16} />
                Rename
              </button>
              <button type="button" role="menuitem" className={styles.item} onClick={() => setMode('move')}>
                <FolderSimple size={16} />
                Move to group
                <CaretRight size={12} className={styles.itemCaret} />
              </button>
              <div className={styles.separator} />
              <button
                type="button"
                role="menuitem"
                className={styles.item}
                onClick={() =>
                  apply(
                    { hidden: !category.hidden },
                    category.hidden ? `${category.name} is visible again` : `${category.name} hidden`,
                  )
                }>
                {category.hidden ? <Eye size={16} /> : <EyeSlash size={16} />}
                {category.hidden ? 'Unhide' : 'Hide'}
              </button>
            </div>
          )}

          {mode === 'rename' && (
            <RenameForm
              label="Rename category"
              initial={category.name}
              onSave={(name) => apply({ name }, 'Category renamed')}
            />
          )}

          {mode === 'move' && (
            <div className={styles.menu}>
              <button type="button" className={styles.back} onClick={() => setMode('menu')}>
                <ArrowLeft size={14} />
                Move {category.name} to
              </button>
              <div className={styles.groupList} role="menu" aria-label="Move to group">
                {destinations.map((group) => {
                  const isCurrent = group.id === category.categoryGroupId;
                  return (
                    <button
                      key={group.id}
                      type="button"
                      role="menuitemradio"
                      aria-checked={isCurrent}
                      className={`${styles.item} ${isCurrent ? styles.current : ''}`}
                      disabled={isCurrent}
                      onClick={() =>
                        apply({ categoryGroupId: group.id ?? '' }, `Moved to ${group.name}`)
                      }>
                      <span className={styles.itemLabel}>{group.name}</span>
                      {isCurrent && <Check size={14} weight="bold" className={styles.itemCaret} />}
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      </Popover>
    </>
  );
}

/** Pencil button that renames a category group in place. */
export function GroupRenameButton({ group }: { group: CategoryGroup }) {
  const dispatch = useAppDispatch();
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const [isOpen, setIsOpen] = useState(false);

  if (group.isSystem || !group.id || group.id === HIDDEN_GROUP_ID) return null;

  const save = async (name: string) => {
    try {
      await dispatch(renameCategoryGroup({ group, name })).unwrap();
      toast.success('Group renamed');
      setIsOpen(false);
    } catch (error) {
      toast.error("Couldn't rename group", { description: errorMessage(error, 'Please try again.') });
    }
  };

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={`${styles.trigger} ${isOpen ? styles.triggerOpen : ''}`}
        aria-label={`Rename ${group.name}`}
        title="Rename group"
        onClick={(e) => {
          e.stopPropagation();
          setIsOpen((open) => !open);
        }}>
        <PencilSimple size={14} />
      </button>
      <Popover
        id={`group-rename-${group.id}`}
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
        triggerRef={triggerRef}
        placement="bottom"
        width={280}>
        <div onClick={(e) => e.stopPropagation()}>
          <RenameForm label="Rename group" initial={group.name} onSave={save} />
        </div>
      </Popover>
    </>
  );
}
