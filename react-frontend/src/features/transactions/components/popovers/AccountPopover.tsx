import { useAppSelector } from '@/app/hooks';
import { useDropdown } from '../../hooks/useDropdown';
import type { Account } from '@/features/accounts/types/account.types';
import styles from './Popover.module.css';
import { Autocomplete, AutocompleteItem } from '@heroui/autocomplete';
import type { TransactionDropdownProps } from './types';
import { useCallback } from 'react';

export function AccountDropdown({ value, onClick, autoFocus, variant = 'inline' }: TransactionDropdownProps) {
  const { budgetAccounts } = useAppSelector((state) => state.accounts);
  const selectedAccount = budgetAccounts.find((account) => account.name === value);
  const filterFn = useCallback(
    (accounts: Account[], query: string) => accounts.filter((account) => account.name.trim().toLowerCase().includes(query)),
    [],
  );

  const { filterQuery, setFilterQuery, filteredItems, filterValues } =
    useDropdown(value, budgetAccounts, filterFn);

  const handleOnClick = (account: Account) => {
    setFilterQuery(account.name);
    onClick(account.id!, account.name);
  };

  const inputClassName = variant === 'form' ? styles.formInput : styles.input;

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
          }
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
          placement: 'bottom-start', // Optional: control placement
        }}
        onInputChange={(value) => filterValues(value)}
        onSelectionChange={(key) => {
          const account = budgetAccounts.find((acc) => acc.id === key);
          if (account) {
            handleOnClick(account);
          }
        }}
        items={filteredItems}
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
        }}>
        {(item) => (
          <AutocompleteItem key={item.id} className={styles.item}>
            {item.name}
          </AutocompleteItem>
        )}
      </Autocomplete>
    </div>
  );
}
