import type { TransactionColumns } from '@/types/common.types';
import type { Transaction } from '../types/transaction.types';
import { AccountDropdown } from './popovers/AccountPopover';
import { PayeeDropdown } from './popovers/PayeePopover';
import { CategoryDropdown } from './popovers/CategoryPopover';
import styles from './Transaction.module.css';

type DropdownColKey = 'accountName' | 'payeeName' | 'categoryName';

const DROPDOWN_CONFIG: Record<
  DropdownColKey,
  {
    component:
    | typeof AccountDropdown
    | typeof PayeeDropdown
    | typeof CategoryDropdown;
    idKey: keyof Transaction;
  }
> = {
  accountName: { component: AccountDropdown, idKey: 'accountId' },
  payeeName: { component: PayeeDropdown, idKey: 'payeeId' },
  categoryName: { component: CategoryDropdown, idKey: 'categoryId' },
};

const INPUT_TYPES: Partial<Record<keyof Transaction, string>> = {
  date: 'date',
  outflow: 'number',
  inflow: 'number',
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
}: Props) {
  const isEditable = col.key !== 'balance';
  const isSelected = selectedTxn?.id === txn.id;

  if (!isEditable || !isSelected) {
    return col.render ? col.render(txn) : txn[col.key];
  }
  // transaction is selected
  const value = selectedTxn?.[col.key] ?? '';

  // Dropdown fields (account, payee, category)
  if (isDropdownColumn(col.key)) {
    const { component: DropdownComponent, idKey } = DROPDOWN_CONFIG[col.key];
    return (
      <DropdownComponent
        value={value.toString()}
        onClick={onSelectChange(idKey, col.key)}
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
      className={styles.input}
    />
  );
}
