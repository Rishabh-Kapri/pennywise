import { useMemo } from 'react';
import { useAppSelector } from '@/app/hooks';
import { BankLogo } from '@/features/accounts/components/BankLogo/BankLogo';
import type { TransactionDropdownProps } from './types';
import { MultiSelectMenu } from './MultiSelectMenu';
import { SearchSelect } from './SearchSelect';

export function AccountDropdown({
  value,
  onClick,
  autoFocus,
  variant = 'inline',
  multiple = false,
  selectedIds = [],
  onChangeMultiple,
  placeholder,
  counts,
}: TransactionDropdownProps) {
  const { allAccounts } = useAppSelector((state) => state.accounts);
  // every account type (loans and tracking too); closed ones can't take new transactions
  const pickableAccounts = useMemo(
    () => allAccounts.filter((account) => !account.deleted && (multiple || !account.closed)),
    [allAccounts, multiple],
  );

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

  if (multiple) {
    return (
      <MultiSelectMenu
        id="account-filter-menu"
        items={accountItems.map(({ trailing, ...item }) => ({ ...item, detail: trailing }))}
        selectedIds={selectedIds}
        onChange={(ids) =>
          onChangeMultiple?.(ids, accountItems.filter((item) => ids.includes(item.id)).map((item) => item.name))
        }
        placeholder={placeholder || 'Select account'}
        noun="accounts"
        counts={counts}
        variant={variant}
      />
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
