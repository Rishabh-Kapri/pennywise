import { CheckIcon, MagnifyingGlassIcon } from '@phosphor-icons/react';
import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { Popover } from '@/components/common/Popover/Popover';
import popoverStyles from '@/components/common/pickers/Picker.module.css';
import styles from '@/components/common/pickers/MultiSelectMenu.module.css';
import type { DropdownVariant } from '@/components/common/pickers/types';

export interface MultiSelectItem {
  id: string;
  name: string;
  leading?: ReactNode;
  /** Muted text after the name (an account's last digits). */
  detail?: string;
  /**
   * Heading the item is listed under in the "Other" section (a category's
   * group); shown muted beside the name in Selected and In these results.
   */
  group?: string;
  /** Extra text search matches (e.g. a tag's stored name behind its label). */
  searchText?: string;
}

interface MultiSelectMenuProps {
  id: string;
  items: MultiSelectItem[];
  selectedIds: string[];
  onChange: (ids: string[]) => void;
  placeholder: string;
  /** Plural, lower case: "payees". */
  noun: string;
  /**
   * How many of the filtered transactions carry each value. When given, those
   * values are listed first under "In these results".
   */
  counts?: Record<string, number>;
  variant?: DropdownVariant;
}

/** Splits items into runs that share a group, keeping their order. */
function groupRuns(items: MultiSelectItem[]): { group?: string; items: MultiSelectItem[] }[] {
  const runs: { group?: string; items: MultiSelectItem[] }[] = [];
  for (const item of items) {
    const last = runs[runs.length - 1];
    if (last && last.group === item.group) last.items.push(item);
    else runs.push({ group: item.group, items: [item] });
  }
  return runs;
}

/**
 * The filter tray's multi-select: what's picked stays pinned at the top with
 * a Clear, then values found in the current results, then everything else.
 */
export function MultiSelectMenu({
  id,
  items,
  selectedIds,
  onChange,
  placeholder,
  noun,
  counts,
  variant = 'form',
}: MultiSelectMenuProps) {
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQuery] = useState('');

  useEffect(() => {
    if (isOpen) searchRef.current?.focus();
  }, [isOpen]);

  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);

  const sections = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const matches = (item: MultiSelectItem) =>
      !needle ||
      item.name.toLowerCase().includes(needle) ||
      Boolean(item.group?.toLowerCase().includes(needle)) ||
      Boolean(item.searchText?.toLowerCase().includes(needle));

    const selected: MultiSelectItem[] = [];
    const inResults: MultiSelectItem[] = [];
    const rest: MultiSelectItem[] = [];
    for (const item of items) {
      if (!matches(item)) continue;
      if (selectedSet.has(item.id)) selected.push(item);
      else if (counts && (counts[item.id] ?? 0) > 0) inResults.push(item);
      else rest.push(item);
    }
    inResults.sort((a, b) => (counts?.[b.id] ?? 0) - (counts?.[a.id] ?? 0) || a.name.localeCompare(b.name));
    return { selected, inResults, rest };
  }, [items, query, selectedSet, counts]);

  const toggle = (itemId: string) =>
    onChange(selectedSet.has(itemId) ? selectedIds.filter((x) => x !== itemId) : [...selectedIds, itemId]);

  const selectedItems = items.filter((item) => selectedSet.has(item.id));
  const triggerClass = variant === 'form' ? popoverStyles.formTrigger : popoverStyles.input;
  const nothingFound = !sections.selected.length && !sections.inResults.length && !sections.rest.length;
  const showHeadings = sections.selected.length > 0 || sections.inResults.length > 0;

  const renderRow = (item: MultiSelectItem, showGroup = true) => {
    const isSelected = selectedSet.has(item.id);
    const count = counts?.[item.id];
    const detail = item.detail ?? (showGroup ? item.group : undefined);
    return (
      <button
        key={item.id}
        type="button"
        role="option"
        aria-selected={isSelected}
        className={`${styles.row} ${isSelected ? styles.rowSelected : ''}`}
        onClick={() => toggle(item.id)}>
        <span className={`${styles.check} ${isSelected ? styles.checkOn : ''}`} aria-hidden="true">
          {isSelected && <CheckIcon size={11} weight="bold" />}
        </span>
        {item.leading && <span className={styles.leading}>{item.leading}</span>}
        <span className={styles.name}>{item.name}</span>
        {detail && <span className={styles.detail}>{detail}</span>}
        {count ? <span className={styles.count}>{count}</span> : null}
      </button>
    );
  };

  return (
    <div className={popoverStyles.popoverContainer}>
      <button
        type="button"
        ref={triggerRef}
        onClick={() => {
          if (!isOpen) setQuery('');
          setIsOpen(!isOpen);
        }}
        className={`${triggerClass} ${popoverStyles.triggerButton} ${styles.trigger} ${isOpen ? popoverStyles.open : ''} ${
          selectedItems.length ? '' : popoverStyles.placeholderText
        }`}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-controls={id}>
        {selectedItems.length ? (
          <>
            <span className={styles.triggerName}>{selectedItems[0].name}</span>
            {selectedItems.length > 1 && <span className={styles.more}>+{selectedItems.length - 1}</span>}
          </>
        ) : (
          placeholder
        )}
      </button>
      <Popover
        id={id}
        isOpen={isOpen}
        triggerRef={triggerRef}
        onClose={() => setIsOpen(false)}
        width={Math.max(triggerRef.current?.offsetWidth ?? 0, 280)}>
        <div className={styles.menu}>
          <div className={styles.searchWrap}>
            <MagnifyingGlassIcon size={15} className={styles.searchIcon} />
            <input
              ref={searchRef}
              className={styles.search}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={`Search ${noun}`}
              aria-label={`Search ${noun}`}
            />
          </div>
          <div className={styles.list} role="listbox" aria-multiselectable="true">
            {nothingFound && <div className={styles.empty}>No {noun} found</div>}

            {sections.selected.length > 0 && (
              <section>
                <div className={styles.heading}>
                  <span>Selected · {selectedIds.length}</span>
                  <button type="button" className={styles.clear} onClick={() => onChange([])}>
                    Clear
                  </button>
                </div>
                {sections.selected.map((item) => renderRow(item))}
              </section>
            )}

            {sections.inResults.length > 0 && (
              <section>
                <div className={styles.heading}>
                  <span>In these results</span>
                </div>
                {sections.inResults.map((item) => renderRow(item))}
              </section>
            )}

            {sections.rest.length > 0 && (
              <section>
                {showHeadings && (
                  <div className={styles.heading}>
                    <span>{counts ? `Other ${noun}` : `All ${noun}`}</span>
                  </div>
                )}
                {groupRuns(sections.rest).map((run, index) =>
                  run.group ? (
                    <div key={`${run.group}-${index}`}>
                      <div className={styles.subheading}>{run.group}</div>
                      {run.items.map((item) => renderRow(item, false))}
                    </div>
                  ) : (
                    run.items.map((item) => renderRow(item))
                  ),
                )}
              </section>
            )}
          </div>
        </div>
      </Popover>
    </div>
  );
}
