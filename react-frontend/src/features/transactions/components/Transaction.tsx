import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { fetchAllTransaction } from '../store';
import { LoadingState } from '@/utils';
import styles from './Transaction.module.css';
import { useHeader } from '@/context/HeaderContext';
import {
  ArrowDown,
  ArrowUp,
  Banknote,
  CalendarDays,
  Plus,
  Search,
} from 'lucide-react';
import { getCurrencyLocaleString, getTodaysDate } from '@/utils/date.utils';
import { selectAccountInfoFromId } from '@/features/accounts/store/accountSlice';
import { TransactionSource, type Transaction } from '../types/transaction.types';
import { TransactionSkeleton } from './TransactionSkeleton';
import {
  allAccountTxnCols,
  specificAccountTxnCols,
} from './TransactionColumns';
import type { TransactionColumns } from '@/types/common.types';
import { List, useDynamicRowHeight } from 'react-window';
import { TransactionRow } from './TransactionRow';
import { selectSelectedBudget } from '@/features/budget';
import { Parser } from 'expr-eval';
import { TransactionMobile } from './TransactionMobile';

interface TransactionHeaderProps {
  name: string;
  balance: number;
  onTxnAdd: () => void;
  searchTerm: string;
  onSearchChange: (value: string) => void;
  mobileFilter: MobileFilter;
  onMobileFilterChange: (value: MobileFilter) => void;
}

const parser = new Parser();
type MobileFilter = 'all' | 'incoming' | 'outgoing' | 'week';

function useIsMobile(breakpoint = 768) {
  const [isMobile, setIsMobile] = useState(() =>
    typeof window !== 'undefined'
      ? window.matchMedia(`(max-width: ${breakpoint}px)`).matches
      : false,
  );

  useEffect(() => {
    const mediaQuery = window.matchMedia(`(max-width: ${breakpoint}px)`);
    const handleChange = () => setIsMobile(mediaQuery.matches);

    handleChange();
    mediaQuery.addEventListener('change', handleChange);

    return () => mediaQuery.removeEventListener('change', handleChange);
  }, [breakpoint]);

  return isMobile;
}

const TransactionHeaderContent = ({
  name,
  balance,
  onTxnAdd,
  searchTerm,
  onSearchChange,
  mobileFilter,
  onMobileFilterChange,
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
            value={searchTerm}
            onChange={(event) => onSearchChange(event.target.value)}
          />
        </div>
      </div>
      <div className={styles.mobileFilterChips}>
        <button
          type="button"
          className={`${styles.filterChip} ${
            mobileFilter === 'incoming' ? styles.activeFilterChip : ''
          }`}
          onClick={() =>
            onMobileFilterChange(
              mobileFilter === 'incoming' ? 'all' : 'incoming',
            )
          }>
          <ArrowDown size={16} />
          <span>Incoming</span>
        </button>
        <button
          type="button"
          className={`${styles.filterChip} ${
            mobileFilter === 'outgoing' ? styles.activeFilterChip : ''
          }`}
          onClick={() =>
            onMobileFilterChange(
              mobileFilter === 'outgoing' ? 'all' : 'outgoing',
            )
          }>
          <ArrowUp size={16} />
          <span>Outgoing</span>
        </button>
        <button
          type="button"
          className={`${styles.filterChip} ${
            mobileFilter === 'week' ? styles.activeFilterChip : ''
          }`}
          onClick={() =>
            onMobileFilterChange(mobileFilter === 'week' ? 'all' : 'week')
          }>
          <CalendarDays size={16} />
          <span>This week</span>
        </button>
      </div>
    </div>
  );
};

function getTransactionDate(txn: Transaction) {
  return new Date(`${txn.date}T00:00:00`);
}

function isThisWeek(txn: Transaction) {
  const today = new Date();
  const startOfWeek = new Date(today);
  startOfWeek.setHours(0, 0, 0, 0);
  startOfWeek.setDate(today.getDate() - today.getDay());

  const endOfWeek = new Date(startOfWeek);
  endOfWeek.setDate(startOfWeek.getDate() + 7);

  const txnDate = getTransactionDate(txn);
  return txnDate >= startOfWeek && txnDate < endOfWeek;
}

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
  const [searchTerm, setSearchTerm] = useState('');
  const [mobileFilter, setMobileFilter] = useState<MobileFilter>('all');
  const isMobile = useIsMobile();

  const rowHeight = useDynamicRowHeight({
    defaultRowHeight: isMobile ? 188 : 63,
  });

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
    (key: keyof Transaction, value: string | number | null) => {
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

  const handleInputBlur = useCallback(
    (key: keyof Transaction, value: string | number) => {
      if (selectedTxn && value) {
        try {
          const expr = parser.parse(value as string);
          const result = expr.evaluate();
          return setSelectedTxn((prev) => {
            if (!prev) {
              return null;
            }
            if (key === 'outflow') {
              handleSelectedTxnChange('inflow', null);
            } else if (key === 'inflow') {
              handleSelectedTxnChange('outflow', null);
            }
            return {
              ...prev,
              [key]: result,
            };
          });
        } catch (err) {
          console.log('handleInputBlur:', err);
        }
      }
    },
    [selectedTxn, setSelectedTxn, handleSelectedTxnChange],
  );

  const addTransaction = () => {
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
        balance: transactions[0]?.balance ?? 0,
        note: '',
        accountName: '',
        accountId: paramId,
        payeeName: '',
        payeeId: '',
        categoryName: '',
        categoryId: '',
        source: TransactionSource.PENNYWISE,
        transferAccountId: null,
        transferTransactionId: null,
        tagIds: [],
      };
      setSelectedTxn(emptyTransaction);
      setSelectedTxnIdx(0);
      return [emptyTransaction, ...transactions];
    }
    return transactions;
  }, [isAddingNew, transactions, selectedBudgetId, paramId]);

  const filteredTransactions = useMemo(() => {
    const normalizedSearch = searchTerm.trim().toLowerCase();

    return displayTransactions.filter((txn) => {
      const matchesSearch =
        normalizedSearch.length === 0 ||
        [
          txn.accountName,
          txn.payeeName,
          txn.categoryName,
          txn.note,
          String(txn.outflow ?? ''),
          String(txn.inflow ?? ''),
        ]
          .filter(Boolean)
          .some((value) => value?.toLowerCase().includes(normalizedSearch));

      if (!matchesSearch) {
        return false;
      }

      if (mobileFilter === 'incoming') {
        return (txn.inflow ?? 0) > 0;
      }

      if (mobileFilter === 'outgoing') {
        return (txn.outflow ?? 0) > 0;
      }

      if (mobileFilter === 'week') {
        return isThisWeek(txn);
      }

      return true;
    });
  }, [displayTransactions, mobileFilter, searchTerm]);

  const rowProps = useMemo(
    () => ({
      paramId,
      transactions: filteredTransactions,
      cols,
      isAddingNew,
      selectedTxn,
      selectedTxnIdx,
      handleTxnSelect,
      handleSelectedTxnChange,
      handleInputBlur,
    }),
    [
      paramId,
      filteredTransactions,
      cols,
      isAddingNew,
      selectedTxn,
      selectedTxnIdx,
      handleTxnSelect,
      handleSelectedTxnChange,
      handleInputBlur,
    ],
  );

  useEffect(() => {
    setHeaderContent(
      <TransactionHeaderContent
        name={accountName}
        balance={accountBal}
        onTxnAdd={addTransaction}
        searchTerm={searchTerm}
        onSearchChange={setSearchTerm}
        mobileFilter={mobileFilter}
        onMobileFilterChange={setMobileFilter}
      />,
    );

    // clear header content when component unmounts
    return () => setHeaderContent(null);
  }, [setHeaderContent, accountName, accountBal, searchTerm, mobileFilter]);

  return (
    <>
      {loading === LoadingState.PENDING && <TransactionSkeleton />}
      {loading === LoadingState.SUCCESS && (
        <div
          className={`${styles.wrapper} ${
            paramId ? styles.specificAccount : styles.allAccounts
          }`}>
          {isMobile ? (
            <TransactionMobile
              transactions={filteredTransactions}
              selectedTransactionId={selectedTxn?.id}
              showAccountName={!paramId}
              onSelectTransaction={handleTxnSelect}
            />
          ) : (
            <>
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
                  rowCount={filteredTransactions.length}
                  rowHeight={rowHeight}
                  rowComponent={TransactionRow}
                  rowProps={rowProps}
                />
              </div>
            </>
          )}
        </div>
      )}
    </>
  );
}
