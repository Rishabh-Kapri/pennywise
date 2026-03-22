import type { TransactionColumns } from '@/types/common.types';
import type { Transaction } from '../types/transaction.types';
import { AccountDropdown } from './popovers/AccountPopover';
import { PayeeDropdown } from './popovers/PayeePopover';
import { CategoryDropdown } from './popovers/CategoryPopover';
import { DateDropdown } from './popovers/DatePopover';
import { TagDropdown } from './popovers/TagPopover';
import { useAppSelector } from '@/app/hooks';
import styles from './Transaction.module.css';
import tagStyles from './popovers/TagPopover.module.css';

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
  onTagsChange?: (tagIds: string[]) => void;
  onBlur?: (key: keyof Transaction, value: string | number) => void;
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
  onTagsChange,
  onBlur,
}: Props) {
  const { allTags } = useAppSelector((state) => state.tags);
  const isEditable = col.key !== 'balance';
  const isSelected = selectedTxn?.id === txn.id;

  // Tags column — needs special handling (array, not a primitive)
  if (col.key === 'tagIds') {
    if (isSelected) {
      return (
        <TagDropdown
          selectedTagIds={selectedTxn?.tagIds ?? []}
          onChange={(tagIds) => onTagsChange?.(tagIds)}
        />
      );
    }
    // Read-only: resolve tag IDs to tag objects for display
    const txnTagIds = txn.tagIds ?? [];
    if (txnTagIds.length === 0) return null;
    const resolvedTags = allTags.filter((t) => txnTagIds.includes(t.id));
    return (
      <div className={tagStyles.tagsContainer}>
        {resolvedTags.map((tag) => (
          <span
            key={tag.id}
            className={tagStyles.tagBadge}
            style={{ backgroundColor: tag.color || '#6366f1' }}>
            {tag.name}
          </span>
        ))}
      </div>
    );
  }

  if (!isEditable || !isSelected) {
    if (col.render) return col.render(txn);
    return txn[col.key];
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
      onBlur={(e) => onBlur?.(col.key, e.target.value)}
      className={styles.input}
    />
  );
}
