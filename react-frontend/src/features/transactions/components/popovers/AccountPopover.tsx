import { useAppSelector } from '@/app/hooks';
import { useDropdown } from '../../hooks/useDropdown';
import type { Account } from '@/features/accounts/types/account.types';
import styles from './Popover.module.css';
import { Autocomplete, AutocompleteItem } from '@heroui/autocomplete';
import { Popover } from '@/components/common/Popover/Popover';
import type { TransactionDropdownProps } from './types';
import { useCallback, useRef, useState, useMemo } from 'react';

export function AccountDropdown({
  value,
  onClick,
  autoFocus,
  variant = 'inline',
  multiple = false,
  selectedIds = [],
  onChangeMultiple,
}: TransactionDropdownProps) {
  const { budgetAccounts } = useAppSelector((state) => state.accounts);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const filterFn = useCallback(
    (accounts: Account[], query: string) =>
      accounts.filter((account) => account.name.trim().toLowerCase().includes(query.toLowerCase())),
    [],
  );

  const filteredItems = useMemo(() => {
    return filterFn(budgetAccounts, searchQuery);
  }, [budgetAccounts, searchQuery, filterFn]);

  // Single select handling
  const selectedAccount = budgetAccounts.find((account) => account.name === value);
  const { filterQuery, setFilterQuery, filteredItems: singleFilteredItems, filterValues } =
    useDropdown(value, budgetAccounts, filterFn);

  const handleOnClick = (account: Account) => {
    setFilterQuery(account.name);
    onClick(account.id!, account.name);
  };

  const handleToggleMultiple = (account: Account) => {
    if (!onChangeMultiple) return;
    const accId = account.id!;
    const isSelected = selectedIds.includes(accId);
    let nextIds: string[];
    let nextNames: string[];

    if (isSelected) {
      nextIds = selectedIds.filter((id) => id !== accId);
      nextNames = budgetAccounts
        .filter((acc) => nextIds.includes(acc.id!))
        .map((acc) => acc.name);
    } else {
      nextIds = [...selectedIds, accId];
      nextNames = budgetAccounts
        .filter((acc) => nextIds.includes(acc.id!))
        .map((acc) => acc.name);
    }
    onChangeMultiple(nextIds, nextNames);
  };

  const triggerClassName = variant === 'form' ? styles.formTrigger : styles.input;
  const inputClassName = variant === 'form' ? styles.formInput : styles.input;

  if (multiple) {
    const displayValue = value || 'Select Account';
    return (
      <div className={styles.popoverContainer}>
        <button
          type="button"
          ref={triggerRef}
          onClick={() => setIsOpen((prev) => !prev)}
          className={`${triggerClassName} ${styles.triggerButton} ${isOpen ? styles.open : ''}`}
          aria-haspopup="true"
          aria-expanded={isOpen}
          aria-controls="account-popover-content"
        >
          {displayValue}
        </button>
        <Popover
          id="account-popover-content"
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
              placeholder="Search accounts"
              aria-label="Search accounts"
            />
          </div>
          <div style={{ maxHeight: '250px', overflowY: 'auto' }}>
            {filteredItems.length > 0 ? (
              filteredItems.map((item) => {
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
              <div className={styles.emptyState}>No accounts found</div>
            )}
          </div>
        </Popover>
      </div>
    );
  }

  // Fallback to existing single select Autocomplete
  return (
    <div className={styles.popoverContainer}>
      <Autocomplete
        inputValue={filterQuery}
        selectedKey={selectedAccount?.id ?? null}
        inputProps={{
          autoFocus,
          classNames: {
            inputWrapper: styles.autocompleteInputWrapper,
            innerWrapper: styles.autocompleteInnerWrapper,
            input: inputClassName,
          },
        }}
        classNames={{
          base: styles.autocompleteBase,
          selectorButton: styles.selectorButton,
          clearButton: styles.clearButton,
        }}
        placeholder="Select Account"
        popoverProps={{
          classNames: {
            base: styles.popoverBase,
            content: styles.popoverContent,
          },
          placement: 'bottom-start',
        }}
        onInputChange={(value) => filterValues(value)}
        onSelectionChange={(key) => {
          const account = budgetAccounts.find((acc) => acc.id === key);
          if (account) {
            handleOnClick(account);
          }
        }}
        items={singleFilteredItems}
        listboxProps={{
          emptyContent: 'No accounts found',
          classNames: {
            base: styles.listboxBase,
            list: styles.listboxList,
          },
          itemClasses: {
            base: styles.item,
            selectedIcon: styles.hideIcon,
          },
        }}
      >
        {(item) => (
          <AutocompleteItem key={item.id} className={styles.item}>
            {item.name}
          </AutocompleteItem>
        )}
      </Autocomplete>
    </div>
  );
}
