import { type CSSProperties, type MouseEvent, useCallback, useEffect, useRef, useState } from 'react';
import type { TransactionColumns } from '@/types/common.types';
import { type Transaction, type ListItem } from '../../types/transaction.types';
import styles from '../Transaction/Transaction.module.css';
import { TransactionCell } from '../TransactionCell';
import type { RowComponentProps } from 'react-window';
import { getCurrencyLocaleString } from '@/utils/date.utils';

const INLINE_EDIT_TRANSACTION_KEYS = new Set<keyof Transaction>([
  'payeeName',
  'date',
  'categoryName',
  'accountName',
  'note',
]);

function isInlineEditableKey(key: keyof Transaction) {
  return INLINE_EDIT_TRANSACTION_KEYS.has(key);
}

function isInsideDialog(target: Node) {
  return target instanceof Element && Boolean(target.closest('[role="dialog"]'));
}

function isInlineEditCell(target: Node) {
  return target instanceof Element && Boolean(target.closest('[data-inline-edit-cell="true"]'));
}

function MonthGroupHeader({ item, style }: { item: Extract<ListItem, { type: 'header' }>; style: CSSProperties }) {
  const { label, stats } = item;

  return (
    <div style={style}>
      <div className={styles.monthGroupHeader}>
        <span className={styles.monthGroupLabel}>{label}</span>
        <span className={styles.monthGroupStats}>
          {stats.count} transaction{stats.count !== 1 ? 's' : ''}
          {stats.totalOutflow > 0 && (
            <span className={styles.statOutflow}>-{getCurrencyLocaleString(stats.totalOutflow)}</span>
          )}
          {stats.totalInflow > 0 && (
            <span className={styles.statInflow}>+{getCurrencyLocaleString(stats.totalInflow)}</span>
          )}
        </span>
      </div>
    </div>
  );
}

export interface Props {
  paramId: string;
  listItems: ListItem[];
  cols: TransactionColumns[];
  isAddingNew: boolean;
  selectedTxn: Transaction | null;
  inlineEditingTxnId: string | null;
  handleTxnSelect: (index: number, txn: Transaction | null) => void;
  handleInlineTxnEdit: (index: number, txn: Transaction | null) => void;
  handleSelectedTxnChange: (key: keyof Transaction, value: Transaction[keyof Transaction]) => void;
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
  inlineEditingTxnId,
  handleTxnSelect,
  handleInlineTxnEdit,
  handleSelectedTxnChange,
  handleInputBlur,
  onAutoSave,
}: RowComponentProps<Props>) {
  const item = listItems[index];
  const [activeInlineEditKey, setActiveInlineEditKey] = useState<keyof Transaction | null>(null);
  const activeInlineEditRef = useRef<HTMLDivElement | null>(null);

  const resetInlineEdit = useCallback(() => {
    setActiveInlineEditKey(null);
    handleInlineTxnEdit(item.type === 'row' ? item.originalIndex : -1, null);
  }, [handleInlineTxnEdit, item]);

  useEffect(() => {
    if (item.type !== 'row' || inlineEditingTxnId !== item.txn.id || !activeInlineEditKey) {
      return;
    }

    const handleOutsideClick = (event: globalThis.MouseEvent) => {
      const target = event.target as Node;
      const isInsideActiveEditor = activeInlineEditRef.current?.contains(target);

      if (!isInsideActiveEditor && !isInsideDialog(target) && !isInlineEditCell(target)) {
        resetInlineEdit();
      }
    };

    document.addEventListener('mousedown', handleOutsideClick);
    return () => document.removeEventListener('mousedown', handleOutsideClick);
  }, [activeInlineEditKey, inlineEditingTxnId, item, resetInlineEdit]);

  useEffect(() => {
    if (item.type === 'row' && inlineEditingTxnId !== item.txn.id) {
      setActiveInlineEditKey(null);
    }
  }, [inlineEditingTxnId, item]);

  // ── Hooks (must be called unconditionally before any early return) ──────────
  const createSelectHandler = useCallback(
    (idKey: keyof Transaction, nameKey: keyof Transaction) => (id: string, name: string) => {
      const isClearingCategory = idKey === 'categoryId' && !id && !name;
      if ((!id || !name) && !isClearingCategory) return;

      const nextId = isClearingCategory ? null : id;
      const nextName = isClearingCategory ? null : name;
      handleSelectedTxnChange(idKey, nextId);
      handleSelectedTxnChange(nameKey, nextName);
      // auto-save for existing transactions after a dropdown selection
      if (!isAddingNew && onAutoSave) {
        if (idKey === 'date') {
          onAutoSave({ date: id });
          resetInlineEdit();
        }
        if (idKey === 'payeeId' || idKey === 'categoryId') {
          onAutoSave({ [idKey]: nextId, [nameKey]: nextName });
          resetInlineEdit();
        }
      }
    },
    [handleSelectedTxnChange, isAddingNew, onAutoSave, resetInlineEdit],
  );

  const handleTagsChange = useCallback(
    (tagIds: string[]) => {
      handleSelectedTxnChange('tagIds', tagIds);
    },
    [handleSelectedTxnChange],
  );

  if (item.type === 'header') {
    return <MonthGroupHeader item={item} style={style} />;
  }

  const { txn, originalIndex } = item;

  const onSelect = (txn: Transaction) => {
    if (selectedTxn?.id === txn.id) return;
    handleTxnSelect(originalIndex, txn);
  };

  const editInlineField = (key: keyof Transaction, txn: Transaction) => (event: MouseEvent<HTMLDivElement>) => {
    if (!isInlineEditableKey(key)) return;
    event.stopPropagation();
    setActiveInlineEditKey(key);
    handleInlineTxnEdit(originalIndex, txn);
  };

  const handleInputChange = (key: keyof Transaction, value: string | number) => {
    if (!selectedTxn || key === 'payeeName' || key === 'categoryName' || key === 'accountName') {
      return;
    }
    handleSelectedTxnChange(key, value);
  };

  const isSelected = selectedTxn?.id === txn.id;
  const isInlineEditing = inlineEditingTxnId === txn.id;
  const isFirst = index === 0 || listItems[index - 1]?.type === 'header';
  const isLast = index === listItems.length - 1 || listItems[index + 1]?.type === 'header';

  const cardClass =
    isFirst && isLast ? styles.txnCardSingle : isFirst ? styles.txnCardFirst : isLast ? styles.txnCardLast : '';

  return (
    <div style={style}>
      <div className={`${styles.txnWrapper} ${cardClass} ${!isLast ? styles.txnDivider : ''}`}>
        <div className={`${styles.txnRow} ${isSelected ? styles.txnRowSelected : ''}`} onClick={() => onSelect(txn)}>
          {cols.map((col) => {
            const isActiveInlineEditCell = isInlineEditing && activeInlineEditKey === col.key;
            const cell = (
              <TransactionCell
                col={col}
                txn={txn}
                selectedTxn={isActiveInlineEditCell ? selectedTxn : null}
                autoFocus={isActiveInlineEditCell}
                onFieldChange={handleInputChange}
                onSelectChange={createSelectHandler}
                onTagsChange={handleTagsChange}
                onBlur={(key, value) => {
                  handleInputBlur(key, value);
                }}
              />
            );

            return (
              <div
                key={`${txn.id}-${col.key}`}
                style={{ ...col.layout }}
                className={col.className?.map((c) => styles[c]).join(' ')}>
                {isInlineEditableKey(col.key) ? (
                  <div
                    data-inline-edit-cell="true"
                    ref={isActiveInlineEditCell ? activeInlineEditRef : null}
                    className={`${styles.inlineEditHitArea} ${col.key === 'date' ? styles.dateInlineEditHitArea : ''}`}
                    onClick={editInlineField(col.key, txn)}>
                    {cell}
                  </div>
                ) : (
                  cell
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
