import { useAppSelector } from '@/app/hooks';
import { useDropdown } from '../../hooks/useDropdown';
import type { Account } from '@/features/accounts/types/account.types';
import styles from './Popover.module.css';
import { Autocomplete, AutocompleteItem } from '@heroui/autocomplete';

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

  const { filterQuery, setFilterQuery, filteredItems, filterValues } =
    useDropdown(value, budgetAccounts, filterFn);

  const handleOnClick = (account: Account) => {
    setFilterQuery(account.name);
    onClick(account.id!, account.name);
  };

  return (
    <div className={styles.popoverContainer}>
      <Autocomplete
        inputProps={{
          classNames: {
            input: styles.input,
          }
        }}
        classNames={{
          selectorButton: styles.selectorButton,
          clearButton: styles.clearButton,
        }}
        placeholder="Select Account"
        value={filterQuery}
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
