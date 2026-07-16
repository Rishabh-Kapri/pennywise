import { CheckIcon, PencilSimpleIcon, TrashIcon, XIcon } from '@phosphor-icons/react';
import { useState } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { createTag, deleteTag, selectAllTags, updateTag } from '@/features/tags/store/tagSlice';
import type { Tag } from '@/features/tags/types/tag.types';
import { TAG_COLORS, getAutoColor } from '@/features/tags/utils';
import settingsStyles from './Settings.module.css';
import styles from './TagSettings.module.css';

function ColorSwatchRow({
  selected,
  onSelect,
}: {
  selected: string;
  onSelect: (color: string) => void;
}) {
  return (
    <div className={styles.swatchRow} role="radiogroup" aria-label="Tag color">
      {TAG_COLORS.map((color) => (
        <button
          key={color}
          type="button"
          role="radio"
          aria-checked={selected === color}
          className={`${styles.swatch} ${selected === color ? styles.swatchSelected : ''}`}
          style={{ backgroundColor: color }}
          onClick={() => onSelect(color)}
          title={color}
        />
      ))}
    </div>
  );
}

function TagRow({ tag }: { tag: Tag }) {
  const dispatch = useAppDispatch();
  const [isEditing, setIsEditing] = useState(false);
  const [isConfirmingDelete, setIsConfirmingDelete] = useState(false);
  const [name, setName] = useState(tag.name);
  const [color, setColor] = useState(tag.color || getAutoColor(tag.name));

  const startEdit = () => {
    setName(tag.name);
    setColor(tag.color || getAutoColor(tag.name));
    setIsEditing(true);
    setIsConfirmingDelete(false);
  };

  const save = async () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    if (trimmed !== tag.name || color !== tag.color) {
      await dispatch(updateTag({ id: tag.id, tag: { ...tag, name: trimmed, color } }));
    }
    setIsEditing(false);
  };

  if (isEditing) {
    return (
      <li className={styles.tagRowEditing}>
        <div className={styles.editControls}>
          <span className={styles.tagChip} style={{ backgroundColor: color }}>
            {name.trim() || tag.name}
          </span>
          <input
            className={styles.nameInput}
            value={name}
            autoFocus
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') save();
              if (e.key === 'Escape') setIsEditing(false);
            }}
            aria-label="Tag name"
          />
          <ColorSwatchRow selected={color} onSelect={setColor} />
        </div>
        <div className={styles.rowActions}>
          <button type="button" className={styles.iconBtn} onClick={save} title="Save" aria-label="Save tag">
            <CheckIcon size={16} />
          </button>
          <button
            type="button"
            className={styles.iconBtn}
            onClick={() => setIsEditing(false)}
            title="Cancel"
            aria-label="Cancel editing">
            <XIcon size={16} />
          </button>
        </div>
      </li>
    );
  }

  return (
    <li className={styles.tagRow}>
      <span className={styles.tagChip} style={{ backgroundColor: tag.color || getAutoColor(tag.name) }}>
        {tag.name}
      </span>
      <div className={styles.rowActions}>
        {isConfirmingDelete ? (
          <>
            <button
              type="button"
              className={styles.deleteConfirmBtn}
              onClick={() => dispatch(deleteTag(tag.id))}>
              Delete?
            </button>
            <button
              type="button"
              className={styles.iconBtn}
              onClick={() => setIsConfirmingDelete(false)}
              title="Cancel"
              aria-label="Cancel delete">
              <XIcon size={16} />
            </button>
          </>
        ) : (
          <>
            <button type="button" className={styles.iconBtn} onClick={startEdit} title="Edit" aria-label="Edit tag">
              <PencilSimpleIcon size={16} />
            </button>
            <button
              type="button"
              className={styles.iconBtn}
              onClick={() => setIsConfirmingDelete(true)}
              title="Delete"
              aria-label="Delete tag">
              <TrashIcon size={16} />
            </button>
          </>
        )}
      </div>
    </li>
  );
}

export function TagSettings() {
  const dispatch = useAppDispatch();
  const allTags = useAppSelector(selectAllTags);
  const [newName, setNewName] = useState('');
  const [newColor, setNewColor] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);

  const nameExists = allTags.some(
    (tag) => tag.name.trim().toLowerCase() === newName.trim().toLowerCase(),
  );

  const handleCreate = async () => {
    const name = newName.trim();
    if (!name || nameExists || isCreating) return;
    setIsCreating(true);
    try {
      await dispatch(createTag({ name, color: newColor ?? getAutoColor(name) })).unwrap();
      setNewName('');
      setNewColor(null);
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <div className={settingsStyles.card}>
      <div className={settingsStyles.cardHeader}>
        <h2>Tags</h2>
        <span>Create, rename, and recolor the tags used on transactions</span>
      </div>

      <div className={styles.createSection}>
        <div className={styles.createRow}>
          <input
            className={styles.nameInput}
            placeholder="New tag name…"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleCreate();
            }}
            aria-label="New tag name"
          />
          <button
            type="button"
            className={styles.createBtn}
            onClick={handleCreate}
            disabled={!newName.trim() || nameExists || isCreating}>
            Add tag
          </button>
        </div>
        <ColorSwatchRow
          selected={newColor ?? (newName.trim() ? getAutoColor(newName.trim()) : '')}
          onSelect={setNewColor}
        />
        {nameExists && <span className={styles.duplicateHint}>A tag with this name already exists</span>}
      </div>

      {allTags.length === 0 ? (
        <div className={styles.emptyState}>
          No tags yet. Create one above, or type a new tag name directly on a transaction.
        </div>
      ) : (
        <ul className={styles.tagList}>
          {allTags.map((tag) => (
            <TagRow key={tag.id} tag={tag} />
          ))}
        </ul>
      )}
    </div>
  );
}
