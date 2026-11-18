import { type KeyboardEvent, type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { Popover } from '@/components/common/Popover/Popover';
import styles from '@/components/common/pickers/Picker.module.css';
import type { DropdownVariant } from '@/components/common/pickers/types';

export interface SearchSelectItem {
  id: string;
  name: string;
  /** Optional icon/logo before the name, in the list and in the field. */
  leading?: ReactNode;
  /** Muted detail after the name in the list (e.g. an account's last digits). */
  trailing?: ReactNode;
}

interface SearchSelectProps {
  value: string;
  items: SearchSelectItem[];
  onSelect: (item: SearchSelectItem) => void;
  placeholder: string;
  emptyText: string;
  autoFocus?: boolean;
  variant?: DropdownVariant;
  /** Popover width in px; defaults to the field's width. */
  menuWidth?: number;
  /**
   * Offers "Add “typed text”" when what's typed matches no item exactly; picking
   * it calls this with the trimmed text. Leave unset to allow existing items only.
   */
  onCreate?: (name: string) => void;
}

const CREATE_ID = '__search_select_create__';

const inputClass: Record<DropdownVariant, string> = {
  inline: styles.input,
  form: styles.formInput,
  ghost: styles.ghostInput,
};

/**
 * Type-to-filter single select shared by the payee and account pickers, so
 * the two behave and look the same in rows, forms and the detail panel.
 * Focusing shows every option; typing narrows it. Leaving without picking
 * puts the saved value back.
 */
export function SearchSelect({
  value,
  items,
  onSelect,
  placeholder,
  emptyText,
  autoFocus,
  variant = 'inline',
  menuWidth,
  onCreate,
}: SearchSelectProps) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const listRef = useRef<HTMLDivElement | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQuery] = useState(value);
  const [isFiltering, setIsFiltering] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);

  useEffect(() => {
    setQuery(value);
    setIsFiltering(false);
  }, [value]);

  const filtered = useMemo(() => {
    const typed = query.trim();
    const needle = typed.toLowerCase();
    if (!isFiltering || !needle) return items;
    const matches = items.filter((item) => item.name.toLowerCase().includes(needle));
    const exact = items.some((item) => item.name.trim().toLowerCase() === needle);
    // last, so Enter on a partial name picks the match, not a new payee
    return onCreate && !exact ? [...matches, { id: CREATE_ID, name: typed }] : matches;
  }, [items, query, isFiltering, onCreate]);

  useEffect(() => {
    if (!isOpen) return;
    const current = filtered.findIndex((item) => item.name === value);
    setActiveIndex(isFiltering || current < 0 ? 0 : current);
  }, [filtered, isOpen, isFiltering, value]);

  useEffect(() => {
    listRef.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' });
  }, [activeIndex, isOpen]);

  const choose = (item: SearchSelectItem) => {
    setQuery(item.name);
    setIsFiltering(false);
    setIsOpen(false);
    if (item.id === CREATE_ID) onCreate?.(item.name);
    else onSelect(item);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      if (!isOpen) setIsOpen(true);
      const step = event.key === 'ArrowDown' ? 1 : -1;
      setActiveIndex((index) => Math.min(Math.max(index + step, 0), Math.max(filtered.length - 1, 0)));
    } else if (event.key === 'Enter') {
      const item = filtered[activeIndex] ?? filtered[0];
      if (!item) return;
      event.preventDefault();
      choose(item);
    } else if (event.key === 'Escape') {
      // close just the list, not the panel or row editor around it
      if (isOpen) event.stopPropagation();
      setIsOpen(false);
      inputRef.current?.blur();
    }
  };

  const selected = items.find((item) => item.name === value);
  // ghost fields are right-aligned text; their owner shows any logo beside them instead
  const showLeading = Boolean(selected?.leading) && !isFiltering && variant !== 'ghost';

  return (
    <div className={`${styles.popoverContainer} ${styles.selectField} ${variant === 'ghost' ? styles.ghostField : ''}`}>
      {showLeading && <span className={styles.selectLeading}>{selected?.leading}</span>}
      <input
        ref={inputRef}
        className={`${inputClass[variant]} ${showLeading ? styles.withLeading : ''}`}
        value={query}
        placeholder={placeholder}
        autoFocus={autoFocus}
        role="combobox"
        aria-expanded={isOpen}
        aria-autocomplete="list"
        onFocus={(event) => {
          setIsOpen(true);
          event.target.select();
        }}
        onBlur={() => {
          setIsOpen(false);
          setQuery(value);
          setIsFiltering(false);
        }}
        onChange={(event) => {
          setQuery(event.target.value);
          setIsFiltering(true);
          setIsOpen(true);
        }}
        onKeyDown={handleKeyDown}
      />
      <Popover
        id={`${placeholder.replace(/\s+/g, '-').toLowerCase()}-options`}
        isOpen={isOpen}
        triggerRef={inputRef}
        width={menuWidth}
        alignment={variant === 'ghost' ? 'end' : 'start'}
        className={styles.optionPopover}>
        <div ref={listRef} role="listbox" className={styles.optionList}>
          {filtered.length === 0 && <div className={styles.optionEmpty}>{emptyText}</div>}
          {filtered.map((item, index) => (
            <div
              key={item.id}
              role="option"
              aria-selected={item.name === value}
              data-active={index === activeIndex}
              className={`${styles.option} ${index === activeIndex ? styles.optionActive : ''} ${item.name === value ? styles.optionSelected : ''}`}
              onMouseEnter={() => setActiveIndex(index)}
              onMouseDown={(event) => {
                // keep focus in the input so blur doesn't close before the pick lands
                event.preventDefault();
                choose(item);
              }}>
              {item.leading && <span className={styles.optionLeading}>{item.leading}</span>}
              <span className={styles.optionName}>
                {item.id === CREATE_ID ? `Add “${item.name}”` : item.name}
              </span>
              {item.trailing && <span className={styles.optionTrailing}>{item.trailing}</span>}
            </div>
          ))}
        </div>
      </Popover>
    </div>
  );
}
