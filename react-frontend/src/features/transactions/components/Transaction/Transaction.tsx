import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import {
  createTransaction,
  deleteTransactionById,
  fetchAllTransaction,
  updateTransaction,
} from '../../store';
import { LoadingState } from '@/utils';
import { toast } from '@/utils';
import styles from './Transaction.module.css';
import { useHeader } from '@/context/HeaderContext';
import { LucideMinus, LucidePlus } from 'lucide-react';
import { getCurrencyLocaleString, getTodaysDate } from '@/utils/date.utils';
import { selectAccountInfoFromId } from '@/features/accounts/store/accountSlice';
import {
  TransactionSource,
  type Transaction,
  type TransactionDTO,
  type ListItem,
  type MonthGroupStats,
} from '../../types/transaction.types';
import { TransactionSkeleton } from '../TransactionSkeleton';
import {
  allAccountTxnCols,
  specificAccountTxnCols,
} from '../TransactionColumns';
import type { TransactionColumns } from '@/types/common.types';
import { List } from 'react-window';
import { TransactionRow } from '../TransactionRow';
import { TransactionDetailPanel } from '../TransactionDetailPanel';
import { selectSelectedBudget } from '@/features/budget';
import { Parser } from 'expr-eval';
import { TransactionMobile } from '../TransactionMobile';
import { TransactionHeader, type MobileFilter } from '../TransactionHeader';

const parser = new Parser();

const HEADER_ROW_HEIGHT = 56;
const TXN_ROW_HEIGHT = 84;

// ── Utilities ────────────────────────────────────────────────────────────────

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

function groupTransactions(txns: Transaction[]): ListItem[] {
  const groups = new Map<string, { label: string; txns: Transaction[] }>();

  for (const txn of txns) {
    const [year, month] = txn.date.split('-');
    const key = `${year}-${month}`;
    if (!groups.has(key)) {
      const d = new Date(Number(year), Number(month) - 1, 1);
      const label = d.toLocaleDateString('en-US', { month: 'long', year: 'numeric' });
      groups.set(key, { label, txns: [] });
    }
    groups.get(key)!.txns.push(txn);
  }

  const items: ListItem[] = [];
  let runningIndex = 0;

  for (const [key, group] of groups) {
    const stats: MonthGroupStats = group.txns.reduce(
      (acc, t) => {
        acc.count++;
        acc.totalInflow += t.inflow ?? 0;
        acc.totalOutflow += t.outflow ?? 0;
        return acc;
      },
      { count: 0, totalInflow: 0, totalOutflow: 0 },
    );
    items.push({ type: 'header', key, label: group.label, stats });
    for (const txn of group.txns) {
      items.push({ type: 'row', txn, originalIndex: runningIndex++ });
    }
  }

  return items;
}

function useIsMobile(breakpoint = 768) {
  const [isMobile, setIsMobile] = useState(() =>
    typeof window !== 'undefined'
      ? window.matchMedia(`(max-width: ${breakpoint}px)`).matches
      : false,
  );
  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${breakpoint}px)`);
    const handle = () => setIsMobile(mq.matches);
    handle();
    mq.addEventListener('change', handle);
    return () => mq.removeEventListener('change', handle);
  }, [breakpoint]);
  return isMobile;
}

// ── Main component ────────────────────────────────────────────────────────────

export function Transaction() {
  const { setHeaderContent } = useHeader();
  const { id } = useParams();
  const paramId = id ?? '';
  const dispatch = useAppDispatch();
  const { loading, transactions } = useAppSelector((state) => state.transactions);
  const { name: accountName, balance: accountBal } = useAppSelector((state) =>
    selectAccountInfoFromId(state, paramId ?? ''),
  );
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const selectedBudgetId = selectedBudget?.id ?? '';
  const [cols, setCols] = useState<TransactionColumns[]>([]);
  const [selectedTxn, setSelectedTxn] = useState<Transaction | null>(null);
  const [isAddingNew, setIsAddingNew] = useState(false);
  const [isPanelClosing, setIsPanelClosing] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [mobileFilter, setMobileFilter] = useState<MobileFilter>('all');
  const [stickyHeader, setStickyHeader] = useState<{ label: string; stats: MonthGroupStats } | null>(null);
  const isMobile = useIsMobile();

  const isPanelOpen = isAddingNew || selectedTxn !== null;
  const isPanelVisible = isPanelOpen || isPanelClosing;

  const rowHeightForItem = useCallback(
    (index: number, items: ListItem[]) => {
      if (isMobile) return 188;
      const item = items[index];
      return item?.type === 'header' ? HEADER_ROW_HEIGHT : TXN_ROW_HEIGHT;
    },
    [isMobile],
  );

  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setSelectedTxn(null);
        setIsAddingNew(false);
      }
    };
    setSelectedTxn(null);
    setIsAddingNew(false);
    if (!paramId) {
      setCols([...allAccountTxnCols]);
      dispatch(fetchAllTransaction(''));
    } else {
      setCols([...specificAccountTxnCols]);
      dispatch(fetchAllTransaction(paramId));
    }
    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [paramId, dispatch]);

  const handleTxnSelect = useCallback((index: number, txn: Transaction | null) => {
    if (isAddingNew) return; // don't overwrite add-new mode
    setIsAddingNew(false);
    setSelectedTxn(txn);
    void index; // originalIndex carried in ListItem, not needed here
  }, [isAddingNew]);

  const resetSelectedTxn = useCallback(() => {
    setIsPanelClosing(true);
  }, []);

  const handlePanelAnimationEnd = useCallback(() => {
    if (isPanelClosing) {
      setIsPanelClosing(false);
      setSelectedTxn(null);
      setIsAddingNew(false);
    }
  }, [isPanelClosing]);

  const handleSelectedTxnChange = useCallback(
    (key: keyof Transaction, value: string | number | null) => {
      setSelectedTxn((prev) => {
        if (!prev) return null;
        return { ...prev, [key]: value };
      });
    },
    [],
  );

  // Build the save payload from a transaction object
  const buildPayload = (txn: Transaction): TransactionDTO => ({
    id: txn.id,
    budgetId: txn.budgetId,
    accountId: txn.accountId,
    payeeId: txn.payeeId,
    categoryId: txn.categoryId === '' ? null : txn.categoryId,
    date: txn.date,
    amount: txn.outflow ? -txn.outflow : (txn.inflow ?? 0),
    note: txn.note ?? '',
    source: TransactionSource.PENNYWISE,
    tagIds: txn.tagIds ?? [],
  });

  // Auto-save for existing transactions after cell blur (amount/note/date)
  const handleInputBlur = useCallback(
    (key: keyof Transaction, value: string | number) => {
      if (!selectedTxn || !value) return;
      try {
        const result = (parser.parse(value as string) as { evaluate(): number }).evaluate();
        // Compute updated txn inline (don't wait for setState)
        let updatedTxn: Transaction = { ...selectedTxn };
        if (key === 'outflow') {
          // Merged amount column: positive → inflow, negative → outflow
          if (result >= 0) {
            updatedTxn = { ...updatedTxn, inflow: result || null, outflow: null };
          } else {
            updatedTxn = { ...updatedTxn, outflow: Math.abs(result), inflow: null };
          }
        } else {
          updatedTxn = { ...updatedTxn, [key]: result };
        }
        setSelectedTxn(updatedTxn);
        // Auto-save existing transactions
        if (!isAddingNew && updatedTxn.id) {
          dispatch(updateTransaction(buildPayload(updatedTxn)))
            .unwrap()
            .then(() => toast.success('Saved'))
            .catch(() => toast.error('Failed to save'));
        }
      } catch (err) {
        console.log('handleInputBlur:', err);
      }
    },
    [selectedTxn, isAddingNew, dispatch],
  );

  // Auto-save after dropdown select in a row cell
  const handleAutoSave = useCallback(
    (overrides: Partial<Transaction>) => {
      if (!selectedTxn || isAddingNew || !selectedTxn.id) return;
      const updatedTxn: Transaction = { ...selectedTxn, ...overrides };
      dispatch(updateTransaction(buildPayload(updatedTxn)))
        .unwrap()
        .then(() => toast.success('Saved'))
        .catch(() => toast.error('Failed to save'));
    },
    [selectedTxn, isAddingNew, dispatch],
  );

  const addTransaction = useCallback(() => {
    const emptyTransaction: Transaction = {
      id: '',
      budgetId: selectedBudgetId,
      date: getTodaysDate(),
      outflow: null,
      inflow: null,
      balance: transactions[0]?.balance ?? 0,
      note: '',
      accountName: '',
      accountId: paramId,
      payeeName: '',
      payeeId: '',
      categoryName: '',
      categoryId: '',
      transferAccountId: null,
      transferTransactionId: null,
      tagIds: [],
    };
    setSelectedTxn(emptyTransaction);
    setIsAddingNew(true);
  }, [selectedBudgetId, transactions, paramId]);

  const handleSave = useCallback(async () => {
    if (!selectedTxn) return;
    try {
      if (isAddingNew) {
        await dispatch(createTransaction(buildPayload(selectedTxn))).unwrap();
        toast.success('Transaction created');
        dispatch(fetchAllTransaction(paramId || ''));
      } else {
        await dispatch(updateTransaction(buildPayload(selectedTxn))).unwrap();
        toast.success('Transaction updated');
      }
      resetSelectedTxn();
    } catch {
      toast.error('Failed to save transaction');
    }
  }, [selectedTxn, isAddingNew, dispatch, paramId, resetSelectedTxn]);

  const handleDelete = useCallback(async () => {
    if (!selectedTxn?.id) return;
    try {
      await dispatch(deleteTransactionById(selectedTxn.id)).unwrap();
      dispatch(fetchAllTransaction(paramId ? selectedTxn.accountId : ''));
      toast.success('Transaction deleted');
      resetSelectedTxn();
    } catch {
      toast.error('Failed to delete transaction');
    }
  }, [selectedTxn, dispatch, paramId, resetSelectedTxn]);

  const handleStatusChange = useCallback(async (status: 'APPROVED' | 'REJECTED') => {
    if (!selectedTxn?.id) return;
    try {
      await dispatch(updateTransaction(buildPayload({ ...selectedTxn, status }))).unwrap();
      dispatch(fetchAllTransaction(paramId ? selectedTxn.accountId : ''));
      toast.success(`Transaction ${status.toLowerCase()}`);
      resetSelectedTxn();
    } catch {
      toast.error('Failed to update status');
    }
  }, [selectedTxn, dispatch, paramId, resetSelectedTxn]);

  const handlePanelSelectChange = useCallback(
    (idKey: keyof Transaction, nameKey: keyof Transaction) =>
      (id: string, name: string) => {
        handleSelectedTxnChange(idKey, id);
        handleSelectedTxnChange(nameKey, name);
      },
    [handleSelectedTxnChange],
  );

  // Filter transactions
  const filteredTransactions = useMemo(() => {
    const search = searchTerm.trim().toLowerCase();
    return transactions.filter((txn) => {
      const matchesSearch =
        !search ||
        [txn.accountName, txn.payeeName, txn.categoryName, txn.note,
          String(txn.outflow ?? ''), String(txn.inflow ?? '')]
          .filter(Boolean)
          .some((v) => v?.toLowerCase().includes(search));
      if (!matchesSearch) return false;
      if (mobileFilter === 'incoming') return (txn.inflow ?? 0) > 0;
      if (mobileFilter === 'outgoing') return (txn.outflow ?? 0) > 0;
      if (mobileFilter === 'week') return isThisWeek(txn);
      return true;
    });
  }, [transactions, searchTerm, mobileFilter]);

  // Group transactions into ListItem[] for react-window
  const listItems = useMemo(() => groupTransactions(filteredTransactions), [filteredTransactions]);

  const rowProps = useMemo(
    () => ({
      paramId,
      listItems,
      cols,
      isAddingNew,
      selectedTxn,
      handleTxnSelect,
      handleSelectedTxnChange,
      handleInputBlur,
      onAutoSave: handleAutoSave,
    }),
    [paramId, listItems, cols, isAddingNew, selectedTxn, handleTxnSelect,
      handleSelectedTxnChange, handleInputBlur, handleAutoSave],
  );

  const dynamicRowHeight = useCallback(
    (index: number) => rowHeightForItem(index, listItems),
    [rowHeightForItem, listItems],
  );

  // Drive sticky header based on the topmost visible row
  const handleRowsRendered = useCallback(
    ({ startIndex }: { startIndex: number }) => {
      // If the topmost visible item is already a header, no overlay needed
      if (listItems[startIndex]?.type === 'header') {
        setStickyHeader(null);
        return;
      }
      // Walk up to find the most recent header above startIndex
      for (let i = startIndex - 1; i >= 0; i--) {
        const item = listItems[i];
        if (item?.type === 'header') {
          setStickyHeader({ label: item.label, stats: item.stats });
          return;
        }
      }
      setStickyHeader(null);
    },
    [listItems],
  );

  useEffect(() => {
    setHeaderContent(
      <TransactionHeader
        name={accountName}
        balance={accountBal}
        onTxnAdd={addTransaction}
        searchTerm={searchTerm}
        onSearchChange={setSearchTerm}
        mobileFilter={mobileFilter}
        onMobileFilterChange={setMobileFilter}
      />,
    );
    return () => setHeaderContent(null);
  }, [setHeaderContent, accountName, accountBal, searchTerm, mobileFilter, addTransaction]);

  return (
    <>
      {loading === LoadingState.PENDING && <TransactionSkeleton />}
      {loading === LoadingState.SUCCESS && (
        <div
          className={`${styles.wrapper} ${paramId ? styles.specificAccount : styles.allAccounts} ${
            isPanelVisible ? styles.panelOpen : ''
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
              {/* Transaction list */}
              <div className={styles.listArea}>
                <div className={styles.headerContainer}>
                  {cols.map((col) => (
                    <div key={col.key} style={{ ...col.layout }}>
                      {col.label}
                    </div>
                  ))}
                </div>
                <div className={styles.txnContainer}>
                  {stickyHeader && (
                    <div className={styles.stickyHeaderOverlay}>
                      <span className={styles.monthGroupLabel}>{stickyHeader.label}</span>
                      <span className={styles.monthGroupStats}>
                        <span className={styles.statCount}>{stickyHeader.stats.count} transaction{stickyHeader.stats.count !== 1 ? 's' : ''}</span>
                        {stickyHeader.stats.totalOutflow > 0 && (
                          <span className={styles.statOutflow}>
                            <LucideMinus color="var(--color-text-secondary)" size={14} />
                            {getCurrencyLocaleString(stickyHeader.stats.totalOutflow)}
                          </span>
                        )}
                        {stickyHeader.stats.totalInflow > 0 && (
                          <span className={styles.statInflow}>
                            <LucidePlus color="var(--color-text-secondary)" size={14} />
                            {getCurrencyLocaleString(stickyHeader.stats.totalInflow)}
                          </span>
                        )}
                      </span>
                    </div>
                  )}
                  <List
                    defaultHeight={500}
                    rowCount={listItems.length}
                    rowHeight={dynamicRowHeight}
                    rowComponent={TransactionRow}
                    rowProps={rowProps}
                    onRowsRendered={handleRowsRendered}
                  />
                </div>
              </div>

              {/* Detail panel */}
              {isPanelVisible && (
                <TransactionDetailPanel
                  selectedTxn={selectedTxn}
                  isAddingNew={isAddingNew}
                  isClosing={isPanelClosing}
                  onChange={handleSelectedTxnChange}
                  onSelectChange={handlePanelSelectChange}
                  onSave={handleSave}
                  onDelete={handleDelete}
                  onClose={resetSelectedTxn}
                  onStatusChange={handleStatusChange}
                  onAnimationEnd={handlePanelAnimationEnd}
                />
              )}
            </>
          )}
        </div>
      )}
    </>

  );
}
