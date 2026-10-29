import { Check } from '@phosphor-icons/react';
import { Popover } from '@/components/common/Popover/Popover';
import { useCallback, useRef, useState, useMemo } from 'react';
import styles from './Popover.module.css';
import { useAppSelector } from '@/app/hooks';
import type { Payee } from '@/features/payees/types/payee.types';
import type { TransactionDropdownProps } from './types';
import { SearchSelect } from './SearchSelect';

export function PayeeDropdown({
  value,
  onClick,
  autoFocus,
  variant = 'inline',
  multiple = false,
  selectedIds = [],
  onChangeMultiple,
  placeholder,
}: TransactionDropdownProps) {
  const { allPayees } = useAppSelector((state) => state.payees);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
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

  const payeeItems = useMemo(
    () => allPayees.filter((payee) => payee.id).map((payee) => ({ id: payee.id!, name: payee.name })),
    [allPayees],
  );

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
    const displayValue = value || placeholder || 'Select Payee';
    return (
      <div className={styles.popoverContainer}>
        <button
          type="button"
          ref={triggerRef}
          onClick={() => setIsOpen((prev) => !prev)}
          className={`${triggerClassName} ${styles.triggerButton} ${isOpen ? styles.open : ''} ${value ? '' : styles.placeholderText}`}
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
          width={Math.max(triggerRef.current?.offsetWidth ?? 0, 240)}
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
          <div className={styles.multiList}>
            {filteredItemsMultiple.length > 0 ? (
              filteredItemsMultiple.map((item) => {
                const isSelected = selectedIds.includes(item.id!);
                return (
                  <div
                    key={item.id}
                    className={`${styles.option} ${styles.multiOption} ${isSelected ? styles.optionSelected : ''}`}
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
                    <span className={styles.optionName}>{item.name}</span>
                    {isSelected && <Check className={styles.optionCheck} size={14} weight="bold" />}
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

  return (
    <SearchSelect
      value={value}
      items={payeeItems}
      onSelect={(item) => onClick(item.id, item.name)}
      placeholder="Select payee"
      emptyText="No payees found"
      autoFocus={autoFocus}
      variant={variant}
      menuWidth={variant === 'ghost' ? 260 : undefined}
    />
  );
}
