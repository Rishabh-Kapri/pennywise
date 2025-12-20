import { useAppSelector } from '@/app/hooks';
import { useDropdown } from '../../hooks/useDropdown';
import type { Account } from '@/features/accounts/types/account.types';
import { useRef } from 'react';
import styles from './Popover.module.css';
import { Popover } from '@/components/common/Popover/Popover';

interface Props {
  value: string;
  onClick: (id: string, name: string) => void;
}

export function AccountDropdown({ value, onClick }: Props) {
  const { budgetAccounts } = useAppSelector((state) => state.accounts);
  const filterFn = (accounts: Account[], query: string) => {
    return accounts.filter((account) =>
      account.name.trim().toLowerCase().includes(query),
    );
  };

  const {
    isOpen,
    setIsOpen,
    filterQuery,
    setFilterQuery,
    filteredItems,
    filterValues,
  } = useDropdown(value, budgetAccounts, filterFn);

  const triggerRef = useRef<HTMLInputElement | null>(null);

  const handleOnBlur = () => {
    setIsOpen(false);
  };

  const handleOnClick = (account: Account) => {
    setIsOpen(false);
    setFilterQuery(account.name);
    onClick(account.id!, account.name);
  };

  return (
    <div className={styles.popoverContainer}>
      <input
        ref={triggerRef}
        onFocus={() => setIsOpen(true)}
        onBlur={handleOnBlur}
        className={`${styles.input} ${styles.trigger}`}
        onChange={(e) => filterValues(e.target.value)}
        value={filterQuery}
        placeholder="Select Account"
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
          <div className={styles.item}>No accounts found</div>
        )}
      </Popover>
    </div>
  );
}
