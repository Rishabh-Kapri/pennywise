import { Check } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import type { Account } from '@/features/accounts/types/account.types';
import styles from './Popover.module.css';
import { Popover } from '@/components/common/Popover/Popover';
import type { TransactionDropdownProps } from './types';
import { SearchSelect } from './SearchSelect';
import { BankLogo } from '@/features/accounts/components/BankLogo/BankLogo';
import { useCallback, useRef, useState, useMemo } from 'react';

export function AccountDropdown({
  value,
  onClick,
  autoFocus,
  variant = 'inline',
  multiple = false,
  selectedIds = [],
  onChangeMultiple,
  placeholder,
}: TransactionDropdownProps) {
  const { allAccounts } = useAppSelector((state) => state.accounts);
  // every account type (loans and tracking too); closed ones can't take new transactions
  const pickableAccounts = useMemo(
    () => allAccounts.filter((account) => !account.deleted && (multiple || !account.closed)),
    [allAccounts, multiple],
  );
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
    return filterFn(pickableAccounts, searchQuery);
  }, [pickableAccounts, searchQuery, filterFn]);

  const accountItems = useMemo(
    () =>
      pickableAccounts
        .filter((account) => account.id)
        .map((account) => ({
          id: account.id!,
          name: account.name,
          leading: <BankLogo name={account.name} bank={account.bank} type={account.type} size="xs" />,
          trailing: account.suffix ? `••${account.suffix}` : undefined,
        })),
    [pickableAccounts],
  );

  const handleToggleMultiple = (account: Account) => {
    if (!onChangeMultiple) return;
    const accId = account.id!;
    const isSelected = selectedIds.includes(accId);
    let nextIds: string[];
    let nextNames: string[];

    if (isSelected) {
      nextIds = selectedIds.filter((id) => id !== accId);
      nextNames = pickableAccounts
        .filter((acc) => nextIds.includes(acc.id!))
        .map((acc) => acc.name);
    } else {
      nextIds = [...selectedIds, accId];
      nextNames = pickableAccounts
        .filter((acc) => nextIds.includes(acc.id!))
        .map((acc) => acc.name);
    }
    onChangeMultiple(nextIds, nextNames);
  };

  const triggerClassName = variant === 'form' ? styles.formTrigger : styles.input;

  if (multiple) {
    const displayValue = value || placeholder || 'Select Account';
    return (
      <div className={styles.popoverContainer}>
        <button
          type="button"
          ref={triggerRef}
          onClick={() => setIsOpen((prev) => !prev)}
          className={`${triggerClassName} ${styles.triggerButton} ${isOpen ? styles.open : ''} ${value ? '' : styles.placeholderText}`}
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
          width={Math.max(triggerRef.current?.offsetWidth ?? 0, 240)}
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
          <div className={styles.multiList}>
            {filteredItems.length > 0 ? (
              filteredItems.map((item) => {
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
                    <span className={styles.optionLeading}><BankLogo name={item.name} bank={item.bank} type={item.type} size="xs" /></span>
                    <span className={styles.optionName}>{item.name}</span>
                    {item.suffix && <span className={styles.optionTrailing}>••{item.suffix}</span>}
                    {isSelected && <Check className={styles.optionCheck} size={14} weight="bold" />}
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

  return (
    <SearchSelect
      value={value}
      items={accountItems}
      onSelect={(item) => onClick(item.id, item.name)}
      placeholder="Select account"
      emptyText="No accounts found"
      autoFocus={autoFocus}
      variant={variant}
      // the table's account column only fits a logo, so its list can't match the field
      menuWidth={variant === 'form' ? undefined : 260}
    />
  );
}
