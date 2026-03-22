import { useCallback } from 'react';
import type { TransactionColumns } from '@/types/common.types';
import { TransactionSource, type Transaction, type TransactionDTO } from '../types/transaction.types';
import styles from './Transaction.module.css';
import { TransactionCell } from './TransactionCell';
import type { RowComponentProps } from 'react-window';
import { useAppDispatch } from '@/app/hooks';
import { toast } from '@/utils';
import {
  createTransaction,
  deleteTransactionById,
  fetchAllTransaction,
  updateTransaction,
} from '../store';

export interface Props {
  paramId: string;
  transactions: Transaction[];
  cols: TransactionColumns[];
  isAddingNew: boolean;
  selectedTxn: Transaction | null;
  selectedTxnIdx: number;
  handleTxnSelect: (index: number, txn: Transaction | null) => void;
  handleSelectedTxnChange: (
    key: keyof Transaction,
    value: string | number,
  ) => void;
  handleInputBlur: (key: keyof Transaction, value: string | number) => void;
}
export function TransactionRow({
  paramId,
  index,
  style,
  transactions,
  cols,
  isAddingNew,
  selectedTxn,
  selectedTxnIdx,
  handleTxnSelect,
  handleSelectedTxnChange,
  handleInputBlur,
}: RowComponentProps<Props>) {
  const onSelect = (index: number, txn: Transaction) => {
    // Don't overwrite if same transaction is already selected (preserves edits)
    if (selectedTxn?.id === txn.id) {
      return;
    }
    handleTxnSelect(index, txn);
  };
  const dispatch = useAppDispatch();

  const txn = transactions[index];

  const resetSelectedTxn = () => {
    handleTxnSelect(-1, null);
  };

  const deleteTransaction = async () => {
    if (!selectedTxn || !selectedTxn.id) {
      return;
    }
    try {
      await dispatch(deleteTransactionById(selectedTxn.id)).unwrap();
      // if paramId is present dipatch fetch transaction for specific account
      if (paramId) {
        dispatch(fetchAllTransaction(selectedTxn.accountId));
      } else {
        dispatch(fetchAllTransaction(''));
      }
      resetSelectedTxn();
      toast.success('Transaction deleted');
    } catch {
      toast.error('Failed to delete transaction');
    }
  };

  const saveTransaction = async () => {
    console.log('saving transaction', selectedTxn, selectedTxnIdx);
    if (selectedTxn) {
      // generate payload
      const amount = selectedTxn.outflow
        ? -selectedTxn.outflow
        : (selectedTxn.inflow ?? 0);
      const payload: TransactionDTO = {
        budgetId: selectedTxn.budgetId,
        accountId: selectedTxn.accountId,
        payeeId: selectedTxn.payeeId,
        categoryId: selectedTxn.categoryId === '' ? null : selectedTxn.categoryId,
        date: selectedTxn.date,
        amount,
        note: selectedTxn.note ?? '',
        source: TransactionSource.PENNYWISE,
        tagIds: selectedTxn.tagIds ?? [],
      };
      console.log('payload:', payload);

      try {
        if (isAddingNew) {
          await dispatch(createTransaction(payload)).unwrap();
          toast.success('Transaction created');
        } else {
          // this is an existing transaction
          payload.id = selectedTxn.id;
          await dispatch(updateTransaction(payload)).unwrap();
          toast.success('Transaction updated');
        }
      } catch {
        toast.error('Failed to save transaction');
      }
      resetSelectedTxn();
    }
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

  const createSelectHandler = useCallback(
    (idKey: keyof Transaction, nameKey: keyof Transaction) =>
      (id: string, name: string) => {
        if (!id || !name) return;
        handleSelectedTxnChange(idKey, id);
        handleSelectedTxnChange(nameKey, name);
      },
    [handleSelectedTxnChange],
  );

  const handleTagsChange = useCallback(
    (tagIds: string[]) => {
      handleSelectedTxnChange('tagIds' as keyof Transaction, tagIds as unknown as string | number);
    },
    [handleSelectedTxnChange],
  );

  return (
    <div style={style}>
      {' '}
      {/* react-window positioning wrapper */}
      <div
        key={txn.id}
        className={`${styles.txnWrapper} ${selectedTxn?.id == txn.id ? styles.selected : ''}`}>
        <div
          className={`${styles.txnRow} ${selectedTxn?.id == txn.id ? styles.selected : ''}`}
          onClick={() => onSelect(index, txn)}>
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
                onBlur={handleInputBlur}
              />
            </div>
          ))}
        </div>
        <hr
          className={`${selectedTxnIdx !== -1 &&
              (selectedTxnIdx === index || selectedTxnIdx - 1 === index)
              ? styles.borderNone
              : styles.divider
            }`}
        />
        {selectedTxn?.id === txn.id && (
          <div className={styles.btnContainer}>
            <button className={styles.cancelBtn} onClick={resetSelectedTxn}>
              Cancel
            </button>
            <button className={styles.deleteBtn} onClick={deleteTransaction}>
              Delete
            </button>
            <button className={styles.saveBtn} onClick={saveTransaction}>
              Save
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
