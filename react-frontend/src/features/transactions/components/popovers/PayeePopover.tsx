import { Popover } from '@/components/common/Popover/Popover';
import { useRef } from 'react';
import styles from './Popover.module.css';
import { useAppSelector } from '@/app/hooks';
import type { Payee } from '@/features/payees/types/payee.types';
import { useDropdown } from '../../hooks/useDropdown';
import type { TransactionDropdownProps } from './types';

/*
 * This component handles rendering the payees list to be shown when adding or editing a transaction
 * The button element is the trigger which will open the popover as a portal
 * I need to handle the onClose function from the parent component as well, we need to close the dropdown when other dropdown is opened
 */
export function PayeeDropdown({ value, onClick, autoFocus, variant = 'inline' }: TransactionDropdownProps) {
  const { allPayees } = useAppSelector((state) => state.payees);
  const {
    isOpen,
    setIsOpen,
    filterQuery,
    setFilterQuery,
    filteredItems,
    filterValues,
  } = useDropdown(value, allPayees, (allPayees, filterQuery) =>
    allPayees.filter((payee) =>
      payee.name.trim().toLowerCase().includes(filterQuery),
    ),
  );

  const triggerRef = useRef<HTMLInputElement | null>(null);

  const handleOnClick = (payee: Payee) => {
    setIsOpen(false);
    setFilterQuery(payee.name);
    onClick(payee.id!, payee.name);
  };

  const triggerClassName = variant === 'form' ? styles.formInput : styles.input;

  return (
    <div className={styles.popoverContainer}>
      <input
        ref={triggerRef}
        onFocus={() => setIsOpen(true)}
        onBlur={() => setIsOpen(false)}
        className={triggerClassName}
        autoFocus={autoFocus}
        onChange={(e) => filterValues(e.target.value)}
        value={filterQuery}
        placeholder="Select Payee"
        aria-haspopup="true"
        aria-expanded={isOpen}
        aria-controls="popover-content"
      />
      <Popover id={'popover-content'} isOpen={isOpen} triggerRef={triggerRef}>
        {filteredItems.length > 0 &&
          filteredItems.map((item) => (
            <div
              key={item.id}
              className={styles.item}
              tabIndex={0}
              role="option"
              onMouseDown={(e) => {
                e.preventDefault(); // Prevents blur from firing
                handleOnClick(item);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  handleOnClick(item);
                }
              }}>
              {item.name}
            </div>
          ))}
        {filteredItems.length === 0 && (
          <div className={styles.item}>No payees found</div>
        )}
      </Popover>
    </div>
  );
}
