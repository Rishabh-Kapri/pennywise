import { useCallback } from 'react';
import type { TransactionColumns } from '@/types/common.types';
import { type Transaction, type ListItem } from '../../types/transaction.types';
import styles from '../Transaction/Transaction.module.css';
import { TransactionCell } from '../TransactionCell';
import type { RowComponentProps } from 'react-window';
import { getCurrencyLocaleString } from '@/utils/date.utils';

export interface Props {
  paramId: string;
  listItems: ListItem[];
  cols: TransactionColumns[];
  isAddingNew: boolean;
  selectedTxn: Transaction | null;
  handleTxnSelect: (index: number, txn: Transaction | null) => void;
  handleSelectedTxnChange: (
    key: keyof Transaction,
    value: string | number | null,
  ) => void;
  handleInputBlur: (key: keyof Transaction, value: string | number) => void;
  onAutoSave?: (overrides: Partial<Transaction>) => void;
}

export function TransactionRow({
  index,
  style,
  listItems,
  cols,
  isAddingNew,
  selectedTxn,
  handleTxnSelect,
  handleSelectedTxnChange,
  handleInputBlur,
  onAutoSave,
}: RowComponentProps<Props>) {
  const item = listItems[index];

  // ── Hooks (must be called unconditionally before any early return) ──────────
  const createSelectHandler = useCallback(
    (idKey: keyof Transaction, nameKey: keyof Transaction) =>
      (id: string, name: string) => {
        if (!id || !name) return;
        handleSelectedTxnChange(idKey, id);
        handleSelectedTxnChange(nameKey, name);
        // auto-save for existing transactions after a dropdown selection
        if (!isAddingNew && onAutoSave) {
          onAutoSave({ [idKey]: id, [nameKey]: name });
        }
      },
    [handleSelectedTxnChange, isAddingNew, onAutoSave],
  );

  const handleTagsChange = useCallback(
    (tagIds: string[]) => {
      handleSelectedTxnChange('tagIds' as keyof Transaction, tagIds as unknown as string | number);
    },
    [handleSelectedTxnChange],
  );

  // ── Month group header ────────────────────────────────────────────────────
  if (item.type === 'header') {
    const { label, stats } = item;
    return (
      <div style={style}>
        <div className={styles.monthGroupHeader}>
          <span className={styles.monthGroupLabel}>{label}</span>
          <span className={styles.monthGroupStats}>
            {stats.count} transaction{stats.count !== 1 ? 's' : ''}
            {stats.totalOutflow > 0 && (
              <span className={styles.statOutflow}>
                -{getCurrencyLocaleString(stats.totalOutflow)}
              </span>
            )}
            {stats.totalInflow > 0 && (
              <span className={styles.statInflow}>
                +{getCurrencyLocaleString(stats.totalInflow)}
              </span>
            )}
          </span>
        </div>
      </div>
    );
  }

  // ── Transaction row ───────────────────────────────────────────────────────
  const { txn, originalIndex } = item;

  const onSelect = (txn: Transaction) => {
    if (selectedTxn?.id === txn.id) return;
    handleTxnSelect(originalIndex, txn);
  };

  const handleInputChange = (
    key: keyof Transaction,
    value: string | number,
  ) => {
    if (
      !selectedTxn ||
      key === 'payeeName' ||
      key === 'categoryName' ||
      key === 'accountName'
    ) {
      return;
    }
    handleSelectedTxnChange(key, value);
  };

  const isSelected = selectedTxn?.id === txn.id;
  const isFirst = index === 0 || listItems[index - 1]?.type === 'header';
  const isLast = index === listItems.length - 1 || listItems[index + 1]?.type === 'header';

  const cardClass =
    isFirst && isLast ? styles.txnCardSingle
    : isFirst ? styles.txnCardFirst
    : isLast ? styles.txnCardLast
    : '';

  return (
    <div style={style}>
      <div className={`${styles.txnWrapper} ${cardClass} ${!isLast ? styles.txnDivider : ''}`}>
        <div
          className={`${styles.txnRow} ${isSelected ? styles.txnRowSelected : ''}`}
          onClick={() => onSelect(txn)}>
          {cols.map((col) => (
            <div
              key={`${txn.id}-${col.key}`}
              style={{ ...col.layout }}
              className={col.className?.map((c) => styles[c]).join(' ')}>
              <TransactionCell
                col={col}
                txn={txn}
                selectedTxn={selectedTxn}
                onFieldChange={handleInputChange}
                onSelectChange={createSelectHandler}
                onTagsChange={handleTagsChange}
                onBlur={(key, value) => {
                  handleInputBlur(key, value);
                }}
              />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

