import { useMemo } from 'react';
import { useAppSelector } from '@/app/hooks';
import type { TransactionDropdownProps } from './types';
import { MultiSelectMenu } from './MultiSelectMenu';
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
  counts,
  onCreate,
}: TransactionDropdownProps) {
  const { allPayees } = useAppSelector((state) => state.payees);

  const payeeItems = useMemo(
    () => allPayees.filter((payee) => payee.id).map((payee) => ({ id: payee.id!, name: payee.name })),
    [allPayees],
  );

  if (multiple) {
    return (
      <MultiSelectMenu
        id="payee-filter-menu"
        items={payeeItems}
        selectedIds={selectedIds}
        onChange={(ids) =>
          onChangeMultiple?.(ids, payeeItems.filter((item) => ids.includes(item.id)).map((item) => item.name))
        }
        placeholder={placeholder || 'Select payee'}
        noun="payees"
        counts={counts}
        variant={variant}
      />
    );
  }

  return (
    <SearchSelect
      value={value}
      items={payeeItems}
      onSelect={(item) => onClick(item.id, item.name)}
      placeholder={placeholder || 'Select payee'}
      emptyText="No payees found"
      onCreate={onCreate}
      autoFocus={autoFocus}
      variant={variant}
      menuWidth={variant === 'ghost' ? 260 : undefined}
    />
  );
}
