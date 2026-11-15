import {
  CheckIcon,
  InfoIcon,
  MagnifyingGlassIcon,
  PencilSimpleIcon,
  PlusIcon,
  TagIcon,
  TrashIcon,
  XIcon,
} from '@phosphor-icons/react';
import { useMemo, useRef, useState } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { Popover } from '@/components/common/Popover/Popover';
import { createTag, deleteTag, selectAllTags, updateTag } from '@/features/tags/store/tagSlice';
import type { Tag } from '@/features/tags/types/tag.types';
import { type DatedTag, TAG_COLORS, getAutoColor, parseDatedTag } from '@/features/tags/utils';
import { errorMessage, toast } from '@/utils';
import settingsStyles from './Settings.module.css';
import styles from './TagSettings.module.css';

const normalize = (name: string) => name.trim().toLowerCase();
const colorOf = (tag: Tag) => tag.color || getAutoColor(tag.name);

/** A colour dot that opens a swatch picker. */
function ColorPicker({
  id,
  color,
  onSelect,
  label,
}: {
  id: string;
  color: string;
  onSelect: (color: string) => void;
  label: string;
}) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={styles.dotBtn}
        onClick={() => setIsOpen((open) => !open)}
        aria-label={label}
        aria-expanded={isOpen}
        title="Change colour">
        <span className={styles.dot} style={{ backgroundColor: color }} />
      </button>
      <Popover id={id} isOpen={isOpen} triggerRef={triggerRef} onClose={() => setIsOpen(false)} width={226}>
        <div className={styles.swatchGrid} role="radiogroup" aria-label="Tag colour">
          {TAG_COLORS.map((swatch) => (
            <button
              key={swatch}
              type="button"
              role="radio"
              aria-checked={swatch === color}
              aria-label={swatch}
              className={styles.swatch}
              style={{ backgroundColor: swatch }}
              onClick={() => {
                onSelect(swatch);
                setIsOpen(false);
              }}>
              {swatch === color && <CheckIcon size={14} weight="bold" />}
            </button>
          ))}
        </div>
      </Popover>
    </>
  );
}

interface TagEntry {
  tag: Tag;
  dated: DatedTag | null;
}

interface TagGroup {
  key: string;
  label: string | null;
  entries: TagEntry[];
}

/** Dated tags by year (newest first, then by month); the rest in "Other", A–Z. */
function groupTags(entries: TagEntry[]): TagGroup[] {
  const byYear = new Map<number, TagEntry[]>();
  const other: TagEntry[] = [];
  for (const entry of entries) {
    if (entry.dated) {
      const list = byYear.get(entry.dated.year) ?? [];
      list.push(entry);
      byYear.set(entry.dated.year, list);
    } else {
      other.push(entry);
    }
  }
  const byName = (a: TagEntry, b: TagEntry) =>
    a.tag.name.localeCompare(b.tag.name, undefined, { sensitivity: 'base' });
  other.sort(byName);

  // No dated tags: one plain list, no headings.
  if (byYear.size === 0) return [{ key: 'all', label: null, entries: other }];

  const groups: TagGroup[] = [...byYear.entries()]
    .sort(([a], [b]) => b - a)
    .map(([year, list]) => ({
      key: String(year),
      label: String(year),
      entries: list.sort((a, b) => b.dated!.month - a.dated!.month || byName(a, b)),
    }));
  if (other.length) groups.push({ key: 'other', label: 'Other', entries: other });
  return groups;
}

const FORMAT_EXAMPLES: [string, string][] = [
  ['goa-dec-2025', 'Goa · Dec 2025'],
  ['new-delhi-march-2026', 'New Delhi · Mar 2026'],
  ['manali_sept_26', 'Manali · Sep 2026'],
  ['jaipur-02-2026', 'Jaipur · Feb 2026'],
];

/** ⓘ next to the title: explains the place-month-year names the list understands. */
function FormatInfo() {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={styles.infoBtn}
        onClick={() => setIsOpen(true)}
        onMouseEnter={() => setIsOpen(true)}
        onMouseLeave={() => setIsOpen(false)}
        aria-label="Tag name formats"
        aria-expanded={isOpen}>
        <InfoIcon size={19} />
      </button>
      <Popover
        id="tag-format-info"
        isOpen={isOpen}
        triggerRef={triggerRef}
        onClose={() => setIsOpen(false)}
        width={340}
        alignment="start">
        <div className={styles.infoBody}>
          <p className={styles.infoTitle}>Dated tags</p>
          <p className={styles.infoText}>
            Name a tag <code className={styles.infoCode}>place-month-year</code> and it's grouped by year and shown as
            place and month.
          </p>
          <ul className={styles.infoExamples}>
            {FORMAT_EXAMPLES.map(([raw, shown]) => (
              <li key={raw}>
                <code className={styles.infoCode}>{raw}</code>
                <span className={styles.infoArrow}>→</span>
                <span>{shown}</span>
              </li>
            ))}
          </ul>
          <ul className={styles.infoRules}>
            <li>Separate parts with <code className={styles.infoCode}>-</code>, <code className={styles.infoCode}>_</code> or a space</li>
            <li>Month: short or full name, or 1–12</li>
            <li>Year: 4 digits, or 2 (26 = 2026)</li>
            <li>Any other name is listed under Other</li>
          </ul>
        </div>
      </Popover>
    </>
  );
}

function TagRow({
  tag,
  dated,
  nameTaken,
}: {
  tag: Tag;
  dated: DatedTag | null;
  nameTaken: (name: string, exceptId: string) => boolean;
}) {
  const dispatch = useAppDispatch();
  const [isEditing, setIsEditing] = useState(false);
  const [isConfirmingDelete, setIsConfirmingDelete] = useState(false);
  const [name, setName] = useState(tag.name);

  const save = async (patch: Partial<Tag>) => {
    try {
      await dispatch(updateTag({ id: tag.id, tag: { ...tag, ...patch } })).unwrap();
    } catch (err) {
      toast.error('Could not update tag', { description: errorMessage(err, 'Please try again.') });
    }
  };

  const trimmed = name.trim();
  const duplicate = trimmed !== '' && normalize(trimmed) !== normalize(tag.name) && nameTaken(trimmed, tag.id);

  const commitRename = async () => {
    if (!trimmed || duplicate) return;
    setIsEditing(false);
    if (trimmed !== tag.name) await save({ name: trimmed });
  };

  const cancelRename = () => {
    setName(tag.name);
    setIsEditing(false);
  };

  const remove = async () => {
    try {
      await dispatch(deleteTag(tag.id)).unwrap();
      toast.success('Tag deleted', { description: `“${tag.name}” was removed.` });
    } catch (err) {
      toast.error('Could not delete tag', { description: errorMessage(err, 'Please try again.') });
      setIsConfirmingDelete(false);
    }
  };

  return (
    <li className={`${styles.row} ${isEditing ? styles.rowEditing : ''}`}>
      {dated && <span className={styles.month}>{dated.monthLabel}</span>}
      <ColorPicker
        id={`tag-color-${tag.id}`}
        color={colorOf(tag)}
        onSelect={(color) => color !== tag.color && save({ color })}
        label={`Change colour of ${tag.name}`}
      />

      {isEditing ? (
        <div className={styles.renameWrap}>
          <input
            className={`${styles.renameInput} ${duplicate ? styles.inputInvalid : ''}`}
            value={name}
            autoFocus
            onFocus={(e) => e.target.select()}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') commitRename();
              if (e.key === 'Escape') cancelRename();
            }}
            aria-label="Tag name"
            aria-invalid={duplicate}
          />
          {duplicate && <span className={styles.hint}>Name already in use</span>}
        </div>
      ) : (
        <button
          type="button"
          className={styles.name}
          onClick={() => {
            setName(tag.name);
            setIsEditing(true);
            setIsConfirmingDelete(false);
          }}
          title={dated ? `${tag.name} · click to rename` : 'Rename'}>
          {dated ? dated.place || tag.name : tag.name}
        </button>
      )}

      <div className={`${styles.actions} ${isEditing || isConfirmingDelete ? styles.actionsVisible : ''}`}>
        {isEditing ? (
          <>
            <button
              type="button"
              className={styles.iconBtn}
              onClick={commitRename}
              disabled={!trimmed || duplicate}
              aria-label="Save name">
              <CheckIcon size={18} />
            </button>
            <button type="button" className={styles.iconBtn} onClick={cancelRename} aria-label="Cancel rename">
              <XIcon size={18} />
            </button>
          </>
        ) : isConfirmingDelete ? (
          <>
            <button type="button" className={styles.dangerBtn} onClick={remove}>
              Delete
            </button>
            <button
              type="button"
              className={styles.iconBtn}
              onClick={() => setIsConfirmingDelete(false)}
              aria-label="Keep tag">
              <XIcon size={18} />
            </button>
          </>
        ) : (
          <>
            <button
              type="button"
              className={styles.iconBtn}
              onClick={() => {
                setName(tag.name);
                setIsEditing(true);
              }}
              aria-label={`Rename ${tag.name}`}
              title="Rename">
              <PencilSimpleIcon size={18} />
            </button>
            <button
              type="button"
              className={`${styles.iconBtn} ${styles.iconBtnDanger}`}
              onClick={() => setIsConfirmingDelete(true)}
              aria-label={`Delete ${tag.name}`}
              title="Delete">
              <TrashIcon size={18} />
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
  const [query, setQuery] = useState('');
  const [pickedColor, setPickedColor] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);

  const trimmed = query.trim();
  const exactMatch = allTags.some((tag) => normalize(tag.name) === normalize(trimmed));
  const canCreate = trimmed !== '' && !exactMatch && !isCreating;
  const newColor = pickedColor ?? getAutoColor(trimmed || 'tag');

  const groups = useMemo(() => {
    const q = normalize(query);
    const entries = allTags
      .map((tag) => ({ tag, dated: parseDatedTag(tag.name) }))
      .filter(({ tag, dated }) => {
        if (!q) return true;
        const haystack = dated ? `${tag.name} ${dated.place} ${dated.monthLabel} ${dated.year}` : tag.name;
        return haystack.toLowerCase().includes(q);
      });
    return entries.length ? groupTags(entries) : [];
  }, [allTags, query]);

  const nameTaken = (name: string, exceptId: string) =>
    allTags.some((tag) => tag.id !== exceptId && normalize(tag.name) === normalize(name));

  const handleCreate = async () => {
    if (!canCreate) return;
    setIsCreating(true);
    try {
      await dispatch(createTag({ name: trimmed, color: newColor })).unwrap();
      setQuery('');
      setPickedColor(null);
    } catch (err) {
      toast.error('Could not create tag', { description: errorMessage(err, 'Please try again.') });
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <div className={`${settingsStyles.card} ${styles.root}`}>
      <div className={styles.header}>
        <div>
          <div className={styles.titleRow}>
            <h2 className={styles.title}>Tags</h2>
            <FormatInfo />
          </div>
          <p className={styles.subtitle}>Label transactions across categories. Click a name to rename it.</p>
        </div>
        {allTags.length > 0 && (
          <span className={styles.count}>
            {allTags.length} {allTags.length === 1 ? 'tag' : 'tags'}
          </span>
        )}
      </div>

      <div className={styles.toolbar}>
        <div className={styles.searchBox}>
          {canCreate ? (
            <ColorPicker id="tag-color-new" color={newColor} onSelect={setPickedColor} label="New tag colour" />
          ) : (
            <MagnifyingGlassIcon size={18} className={styles.searchIcon} />
          )}
          <input
            className={styles.searchInput}
            placeholder={allTags.length ? 'Search or add a tag' : 'Add your first tag'}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleCreate();
              if (e.key === 'Escape') setQuery('');
            }}
            aria-label="Search or add a tag"
          />
          {query && (
            <button type="button" className={styles.clearBtn} onClick={() => setQuery('')} aria-label="Clear">
              <XIcon size={16} />
            </button>
          )}
        </div>
        <button type="button" className={styles.addBtn} onClick={handleCreate} disabled={!canCreate}>
          <PlusIcon size={17} weight="bold" />
          <span>Add</span>
        </button>
      </div>

      {allTags.length === 0 ? (
        <div className={styles.empty}>
          <TagIcon size={36} className={styles.emptyIcon} />
          <p className={styles.emptyTitle}>No tags yet</p>
          <p className={styles.emptyText}>Add one above, or type a new tag straight onto a transaction.</p>
        </div>
      ) : groups.length === 0 ? (
        <div className={styles.empty}>
          <p className={styles.emptyText}>
            No tag matches “{trimmed}”. Press <kbd className={styles.kbd}>Enter</kbd> to add it.
          </p>
        </div>
      ) : (
        <div className={styles.list}>
          {groups.map((group) => (
            <section key={group.key} className={styles.group}>
              {group.label && (
                <h3 className={styles.groupHeader}>
                  <span>{group.label}</span>
                  <span className={styles.groupCount}>{group.entries.length}</span>
                </h3>
              )}
              <ul className={styles.groupList}>
                {group.entries.map(({ tag, dated }) => (
                  <TagRow key={tag.id} tag={tag} dated={dated} nameTaken={nameTaken} />
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
