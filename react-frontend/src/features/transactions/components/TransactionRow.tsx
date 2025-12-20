import type { TransactionColumns } from '@/types/common.types';
import type { Transaction } from '../types/transaction.types';
import { useCallback } from 'react';
import styles from './Transaction.module.css';
import { TransactionCell } from './TransactionCell';
import type { RowComponentProps } from 'react-window';
import { useAppDispatch } from '@/app/hooks';
import { deleteTransactionById, fetchAllTransaction } from '../store';

export interface Props {
  paramId: string;
  transactions: Transaction[];
  cols: TransactionColumns[];
  selectedTxn: Transaction | null;
  selectedTxnIdx: number;
  handleTxnSelect: (index: number, txn: Transaction | null) => void;
  handleSelectedTxnChange: (
    key: keyof Transaction,
    value: string | number,
  ) => void;
}
export function TransactionRow({
  paramId,
  index,
  style, // CRITICAL: positioning from react-window
  transactions,
  cols,
  selectedTxn,
  selectedTxnIdx,
  handleTxnSelect,
  handleSelectedTxnChange,
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
    await dispatch(deleteTransactionById(selectedTxn.id));
    // if paramId is present dipatch fetch transaction for specific account
    if (paramId) {
      dispatch(fetchAllTransaction(selectedTxn.accountId));
    } else {
      dispatch(fetchAllTransaction(''));
    }
    resetSelectedTxn();
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
            <button className={styles.saveBtn}>Save</button>
          </div>
        )}
      </div>
    </div>
  );
}
