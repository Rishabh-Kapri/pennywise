import { Popover } from '@/components/common/Popover/Popover';
import { useCallback, useRef, useState } from 'react';
import styles from './Popover.module.css';
import tagStyles from './TagPopover.module.css';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import type { Tag } from '@/features/tags/types/tag.types';
import { createTag } from '@/features/tags/store/tagSlice';
import { getAutoColor } from '@/features/tags/utils';
import { useDropdown } from '../../hooks/useDropdown';

interface TagPopoverProps {
  selectedTagIds: string[];
  onChange: (tagIds: string[]) => void;
  allowCreate?: boolean;
  placeholder?: string;
}

export function TagDropdown({
  selectedTagIds,
  onChange,
  allowCreate = true,
  placeholder = 'Search or create tags',
}: TagPopoverProps) {
  const { allTags } = useAppSelector((state) => state.tags);
  const dispatch = useAppDispatch();
  const [isCreating, setIsCreating] = useState(false);
  const filterTags = useCallback(
    (tags: Tag[], query: string) => tags.filter((tag) => tag.name.trim().toLowerCase().includes(query)),
    [],
  );

  const {
    isOpen,
    setIsOpen,
    filterQuery,
    filteredItems,
    filterValues,
  } = useDropdown('', allTags, filterTags);

  const triggerRef = useRef<HTMLInputElement | null>(null);

  const selectedSet = new Set(selectedTagIds);

  const toggleTag = (tag: Tag) => {
    const newSet = new Set(selectedSet);
    if (newSet.has(tag.id)) {
      newSet.delete(tag.id);
    } else {
      newSet.add(tag.id);
    }
    onChange(Array.from(newSet));
  };

  const handleCreateTag = async () => {
    const name = filterQuery.trim();
    if (!name || isCreating) return;
    setIsCreating(true);
    try {
      const newTag = await dispatch(
        createTag({ name, color: getAutoColor(name) }),
      ).unwrap();
      onChange([...selectedTagIds, newTag.id]);
      filterValues('');
    } finally {
      setIsCreating(false);
    }
  };

  const exactMatch = filteredItems.some(
    (t) => t.name.trim().toLowerCase() === filterQuery.trim().toLowerCase(),
  );

  return (
    <div className={styles.popoverContainer}>
      <input
        ref={triggerRef}
        onFocus={() => setIsOpen(true)}
        onBlur={() => setIsOpen(false)}
        className={`${styles.input} ${styles.trigger}`}
        onChange={(e) => filterValues(e.target.value)}
        value={filterQuery}
        placeholder={placeholder}
        aria-haspopup="true"
        aria-expanded={isOpen}
        aria-controls="tag-popover-content"
      />
      <Popover id="tag-popover-content" isOpen={isOpen} triggerRef={triggerRef}>
        {filteredItems.map((tag) => (
          <div
            key={tag.id}
            className={`${styles.item} ${tagStyles.tagItem}`}
            tabIndex={0}
            role="option"
            aria-selected={selectedSet.has(tag.id)}
            onMouseDown={(e) => {
              e.preventDefault();
              toggleTag(tag);
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                toggleTag(tag);
              }
            }}>
            <input
              type="checkbox"
              checked={selectedSet.has(tag.id)}
              readOnly
              className={tagStyles.checkbox}
            />
            <span
              className={tagStyles.tagChip}
              style={{ backgroundColor: tag.color || '#6366f1' }}>
              {tag.name}
            </span>
          </div>
        ))}
        {allowCreate && !exactMatch && filterQuery.trim() && (
          <div
            className={`${styles.item} ${tagStyles.createItem}`}
            tabIndex={0}
            role="option"
            onMouseDown={(e) => {
              e.preventDefault();
              handleCreateTag();
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                handleCreateTag();
              }
            }}>
            + Create &quot;{filterQuery.trim()}&quot;
          </div>
        )}
        {filteredItems.length === 0 && !filterQuery.trim() && (
          <div className={styles.item}>No tags yet</div>
        )}
      </Popover>
    </div>
  );
}
