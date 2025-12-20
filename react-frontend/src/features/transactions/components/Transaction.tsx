import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { fetchAllTransaction } from '../store';
import { LoadingState } from '@/utils';
import styles from './Transaction.module.css';
import { useHeader } from '@/context/HeaderContext';
import { Banknote, Plus, Search } from 'lucide-react';
import { getCurrencyLocaleString, getTodaysDate } from '@/utils/date.utils';
import { selectAccountInfoFromId } from '@/features/accounts/store/accountSlice';
import type { Transaction } from '../types/transaction.types';
import { TransactionSkeleton } from './TransactionSkeleton';
import {
  allAccountTxnCols,
  specificAccountTxnCols,
} from './TransactionColumns';
import type { TransactionColumns } from '@/types/common.types';
import { List, useDynamicRowHeight } from 'react-window';
import { TransactionRow } from './TransactionRow';
import { selectSelectedBudget } from '@/features/budget';

interface TransactionHeaderProps {
  name: string;
  balance: number;
  onTxnAdd: () => void;
}

const TransactionHeaderContent = ({
  name,
  balance,
  onTxnAdd,
}: TransactionHeaderProps) => {
  return (
    <div className={styles.container}>
      <h2 className={styles.title}>
        <Banknote size={30} />
        <span>{name}</span>
      </h2>
      <div
        className={
          balance < 0
            ? `${styles.negative} ${styles.amount}`
            : `${styles.amount}`
        }>
        <h3>{getCurrencyLocaleString(balance)}</h3>
      </div>
      <div className={styles.actionContainer}>
        <div className={styles.addButton} onClick={onTxnAdd}>
          <Plus size={18} />
          <span>Add Expense</span>
        </div>
        <div className={styles.searchContainer}>
          <Search size={18} />
          <input
            type="text"
            className={styles.searchInput}
            placeholder="Search Transactions"
          />
        </div>
      </div>
    </div>
  );
};

export function Transaction() {
  const { setHeaderContent } = useHeader();

  const { id } = useParams();
  const paramId = id ?? '';
  const dispatch = useAppDispatch();
  const { loading, transactions } = useAppSelector(
    (state) => state.transactions,
  );
  const { name: accountName, balance: accountBal } = useAppSelector((state) =>
    selectAccountInfoFromId(state, paramId ?? ''),
  );
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const selectedBudgetId = selectedBudget?.id ?? '';
  const [cols, setCols] = useState<TransactionColumns[]>([]);
  const [selectedTxn, setSelectedTxn] = useState<Transaction | null>(null);
  const [selectedTxnIdx, setSelectedTxnIdx] = useState(-1);
  const [isAddingNew, setIsAddingNew] = useState(false);

  const rowHeight = useDynamicRowHeight({ defaultRowHeight: 63 });

  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        resetSelectedTxn();
        setIsAddingNew(false);
      }
    };
    resetSelectedTxn();

    if (!paramId) {
      setCols([...allAccountTxnCols]);
      dispatch(fetchAllTransaction(''));
    } else {
      setCols([...specificAccountTxnCols]);
      dispatch(fetchAllTransaction(paramId));
    }

    document.addEventListener('keydown', handleEscape);

    return () => {
      document.removeEventListener('keydown', handleEscape);
    };
  }, [paramId, dispatch]);

  const handleTxnSelect = useCallback(
    (index: number, txn: Transaction | null) => {
      // user selects transaction, set new transaction selection to false
      setIsAddingNew(false);
      setSelectedTxn(txn);
      setSelectedTxnIdx(index);
    },
    [],
  );

  const resetSelectedTxn = () => {
    setSelectedTxn(null);
    setSelectedTxnIdx(-1);
  };

  const handleSelectedTxnChange = useCallback(
    (key: keyof Transaction, value: string | number) => {
      return setSelectedTxn((prev) => {
        if (!prev) {
          return null;
        }
        return {
          ...prev,
          [key]: value,
        };
      });
    },
    [setSelectedTxn],
  );

  const addTransaction = () => {
    console.log('Adding Transaction');
    setIsAddingNew(true);
  };

  const displayTransactions = useMemo(() => {
    if (isAddingNew) {
      const emptyTransaction: Transaction = {
        id: '',
        budgetId: selectedBudgetId,
        date: getTodaysDate(),
        outflow: 0,
        inflow: null,
        balance: transactions[0].balance,
        note: '',
        accountName: '',
        accountId: '',
        payeeName: '',
        payeeId: '',
        categoryName: '',
        categoryId: '',
        transferAccountId: null,
        transferTransactionId: null,
      };
      setSelectedTxn(emptyTransaction);
      setSelectedTxnIdx(0);
      return [emptyTransaction, ...transactions];
    }
    return transactions;
  }, [isAddingNew, transactions, selectedBudgetId]);

  const rowProps = useMemo(
    () => ({
      paramId,
      transactions: displayTransactions,
      cols,
      selectedTxn,
      selectedTxnIdx,
      handleTxnSelect,
      handleSelectedTxnChange,
    }),
    [
      paramId,
      displayTransactions,
      cols,
      selectedTxn,
      selectedTxnIdx,
      handleTxnSelect,
      handleSelectedTxnChange,
    ],
  );

  useEffect(() => {
    setHeaderContent(
      <TransactionHeaderContent
        name={accountName}
        balance={accountBal}
        onTxnAdd={addTransaction}
      />,
    );

    // clear header content when component unmounts
    return () => setHeaderContent(null);
  }, [setHeaderContent, accountName, accountBal]);

  return (
    <>
      {loading === LoadingState.PENDING && <TransactionSkeleton />}
      {loading === LoadingState.SUCCESS && (
        <div className={styles.wrapper}>
          <div className={styles.headerContainer}>
            {cols.map((col) => (
              <div key={col.key} style={{ ...col.layout }}>
                {col.label}
              </div>
            ))}
          </div>
          <div className={styles.txnContainer}>
            <List
              defaultHeight={500}
              rowCount={displayTransactions.length}
              rowHeight={rowHeight}
              rowComponent={TransactionRow}
              rowProps={rowProps}
            />
          </div>
        </div>
      )}
    </>
  );
}
