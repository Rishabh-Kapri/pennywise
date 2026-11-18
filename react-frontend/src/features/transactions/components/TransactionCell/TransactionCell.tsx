import type { TransactionColumns } from '@/types/common.types';
import type { Transaction } from '../../types/transaction.types';
import { AccountDropdown } from '@/features/accounts/components/AccountDropdown';
import { PayeeDropdown } from '@/features/payees/components/PayeeDropdown';
import { CategoryDropdown } from '@/features/category/components/CategoryDropdown';
import { DateDropdown } from '@/components/common/pickers/DateDropdown';
import styles from '../Transaction/Transaction.module.css';

type DropdownColKey = 'accountName' | 'payeeName' | 'categoryName' | 'date';

const DROPDOWN_CONFIG: Record<
  DropdownColKey,
  {
    component:
    | typeof AccountDropdown
    | typeof PayeeDropdown
    | typeof CategoryDropdown
    | typeof DateDropdown;
    idKey: keyof Transaction;
  }
> = {
  accountName: { component: AccountDropdown, idKey: 'accountId' },
  payeeName: { component: PayeeDropdown, idKey: 'payeeId' },
  categoryName: { component: CategoryDropdown, idKey: 'categoryId' },
  date: { component: DateDropdown, idKey: 'date' },
};

const INPUT_TYPES: Partial<Record<keyof Transaction, string>> = {
  date: 'date',
  outflow: 'text',
  inflow: 'text',
  note: 'text',
};

function isDropdownColumn(key: keyof Transaction): key is DropdownColKey {
  return key in DROPDOWN_CONFIG;
}

interface Props {
  col: TransactionColumns;
  txn: Transaction;
  selectedTxn: Transaction | null;
  onFieldChange: (key: keyof Transaction, value: string | number) => void;
  onSelectChange: (
    idKey: keyof Transaction,
    nameKey: keyof Transaction,
  ) => (id: string, name: string) => void;
  onBlur?: (key: keyof Transaction, value: string | number) => void;
  autoFocus?: boolean;
}

/**
 * TransactionCell component is used to render individual transaction column cell,
 */
export function TransactionCell({
  col,
  txn,
  selectedTxn,
  onFieldChange,
  onSelectChange,
  onBlur,
  autoFocus,
}: Props) {
  const isEditable = col.key !== 'balance';
  const isSelected = selectedTxn?.id === txn.id;

  if (!isEditable || !isSelected) {
    if (col.render) return col.render(txn);
    return txn[col.key];
  }

  // transaction is selected
  const rawValue = selectedTxn?.[col.key] ?? '';
  const value = typeof rawValue === 'string' || typeof rawValue === 'number' ? rawValue : '';

  // Dropdown fields (account, payee, category)
  if (isDropdownColumn(col.key)) {
    const { component: DropdownComponent, idKey } = DROPDOWN_CONFIG[col.key];
    return (
      <DropdownComponent
        value={value.toString()}
        onClick={onSelectChange(idKey, col.key)}
        autoFocus={autoFocus}
      />
    );
  }
  // regular input fields
  return (
    <input
      type={INPUT_TYPES[col.key] ?? 'text'}
      value={value}
      placeholder={col.label}
      onChange={(e) => onFieldChange(col.key, e.target.value)}
      onBlur={(e) => onBlur?.(col.key, e.target.value)}
      className={styles.input}
      autoFocus={autoFocus}
    />
  );
}
