import { useMemo } from 'react';
import { BankLogo } from '@/features/accounts/components/BankLogo/BankLogo';
import type { Account } from '@/features/accounts/types/account.types';
import { SearchSelect, type SearchSelectItem } from '@/components/common/pickers/SearchSelect';

/**
 * The account a statement (or one of its sections) belongs to: the app's
 * searchable picker with bank logos and last digits. `noneLabel` adds a first
 * option that picks no account.
 */
export function AccountPicker({
  accounts,
  value,
  onChange,
  noneLabel,
  placeholder = 'Choose an account',
  disabled,
}: {
  accounts: Account[];
  value: string;
  onChange: (accountId: string) => void;
  noneLabel?: string;
  placeholder?: string;
  disabled?: boolean;
}) {
  const items = useMemo<SearchSelectItem[]>(
    () => [
      ...(noneLabel ? [{ id: '', name: noneLabel }] : []),
      ...accounts
        .filter((a) => a.id)
        .map((a) => ({
          id: a.id!,
          name: a.name,
          leading: <BankLogo name={a.name} bank={a.bank} type={a.type} size="xs" />,
          trailing: a.suffix ? `••${a.suffix}` : undefined,
        })),
    ],
    [accounts, noneLabel],
  );
  const selected = items.find((item) => item.id === value);

  return (
    <div aria-disabled={disabled} style={disabled ? { pointerEvents: 'none', opacity: 0.6 } : undefined}>
      <SearchSelect
        variant="form"
        value={selected?.name ?? ''}
        items={items}
        placeholder={placeholder}
        emptyText="No accounts match"
        onSelect={(item) => item.id !== value && onChange(item.id)}
      />
    </div>
  );
}
