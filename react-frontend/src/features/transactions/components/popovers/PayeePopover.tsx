import { Popover } from '@/components/common/Popover/Popover';
import { type KeyboardEvent, useCallback, useRef, useState, useMemo } from 'react';
import styles from './Popover.module.css';
import { useAppSelector } from '@/app/hooks';
import type { Payee } from '@/features/payees/types/payee.types';
import { useDropdown } from '../../hooks/useDropdown';
import type { TransactionDropdownProps } from './types';

export function PayeeDropdown({
  value,
  onClick,
  autoFocus,
  variant = 'inline',
  multiple = false,
  selectedIds = [],
  onChangeMultiple,
}: TransactionDropdownProps) {
  const { allPayees } = useAppSelector((state) => state.payees);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const inputTriggerRef = useRef<HTMLInputElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const filterPayees = useCallback(
    (payees: Payee[], filterQuery: string) =>
      payees.filter((payee) => payee.name.trim().toLowerCase().includes(filterQuery.toLowerCase())),
    [],
  );

  const filteredItemsMultiple = useMemo(() => {
    return filterPayees(allPayees, searchQuery);
  }, [allPayees, searchQuery, filterPayees]);

  // Single select handling
  const {
    isOpen: isSingleOpen,
    setIsOpen: setSingleIsOpen,
    filterQuery,
    setFilterQuery,
    filteredItems: singleFilteredItems,
    filterValues,
  } = useDropdown(value, allPayees, filterPayees);

  const handleOnClick = (payee: Payee) => {
    setSingleIsOpen(false);
    setFilterQuery(payee.name);
    onClick(payee.id!, payee.name);
  };

  const handleInputKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter') return;

    const firstPayee = singleFilteredItems[0];
    if (!firstPayee) return;

    e.preventDefault();
    handleOnClick(firstPayee);
  };

  const handleToggleMultiple = (payee: Payee) => {
    if (!onChangeMultiple) return;
    const payeeId = payee.id!;
    const isSelected = selectedIds.includes(payeeId);
    let nextIds: string[];
    let nextNames: string[];

    if (isSelected) {
      nextIds = selectedIds.filter((id) => id !== payeeId);
      nextNames = allPayees
        .filter((p) => nextIds.includes(p.id!))
        .map((p) => p.name);
    } else {
      nextIds = [...selectedIds, payeeId];
      nextNames = allPayees
        .filter((p) => nextIds.includes(p.id!))
        .map((p) => p.name);
    }
    onChangeMultiple(nextIds, nextNames);
  };

  const triggerClassName = variant === 'form' ? styles.formTrigger : styles.input;

  if (multiple) {
    const displayValue = value || 'Select Payee';
    return (
      <div className={styles.popoverContainer}>
        <button
          type="button"
          ref={triggerRef}
          onClick={() => setIsOpen((prev) => !prev)}
          className={`${triggerClassName} ${styles.triggerButton} ${isOpen ? styles.open : ''}`}
          aria-haspopup="true"
          aria-expanded={isOpen}
          aria-controls="payee-popover-content"
        >
          {displayValue}
        </button>
        <Popover
          id="payee-popover-content"
          isOpen={isOpen}
          triggerRef={triggerRef}
          onClose={() => setIsOpen(false)}
        >
          <div className={styles.searchContainer}>
            <input
              ref={searchRef}
              className={styles.searchInput}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search payees"
              aria-label="Search payees"
            />
          </div>
          <div style={{ maxHeight: '250px', overflowY: 'auto' }}>
            {filteredItemsMultiple.length > 0 ? (
              filteredItemsMultiple.map((item) => {
                const isSelected = selectedIds.includes(item.id!);
                return (
                  <div
                    key={item.id}
                    className={`${styles.item} ${isSelected ? styles.selectedItem : ''}`}
                    onClick={() => handleToggleMultiple(item)}
                    role="option"
                    aria-selected={isSelected}
                    tabIndex={0}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        handleToggleMultiple(item);
                      }
                    }}
                  >
                    {item.name}
                  </div>
                );
              })
            ) : (
              <div className={styles.emptyState}>No payees found</div>
            )}
          </div>
        </Popover>
      </div>
    );
  }

  // Fallback to existing single select behavior
  const singleInputClassName = variant === 'form' ? styles.formInput : styles.input;

  return (
    <div className={styles.popoverContainer}>
      <input
        ref={inputTriggerRef}
        onFocus={() => setSingleIsOpen(true)}
        onBlur={() => setSingleIsOpen(false)}
        className={singleInputClassName}
        autoFocus={autoFocus}
        onChange={(e) => filterValues(e.target.value)}
        onKeyDown={handleInputKeyDown}
        value={filterQuery}
        placeholder="Select Payee"
        aria-haspopup="true"
        aria-expanded={isSingleOpen}
        aria-controls="popover-content"
      />
      <Popover id="popover-content" isOpen={isSingleOpen} triggerRef={inputTriggerRef}>
        {singleFilteredItems.length > 0 &&
          singleFilteredItems.map((item) => (
            <div
              key={item.id}
              className={styles.item}
              tabIndex={0}
              role="option"
              onMouseDown={(e) => {
                e.preventDefault();
                handleOnClick(item);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  handleOnClick(item);
                }
              }}
            >
              {item.name}
            </div>
          ))}
        {singleFilteredItems.length === 0 && (
          <div className={styles.item}>No payees found</div>
        )}
      </Popover>
    </div>
  );
}
