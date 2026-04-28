import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { createTransaction, deleteTransactionById, fetchAllTransaction, updateTransaction, updateTransactionStatus } from '../../store';
import { LoadingState } from '@/utils';
import { toast } from '@/utils';
import styles from './Transaction.module.css';
import { useHeader } from '@/context/HeaderContext';
import { useSidePanel } from '@/context/SidePanelContext';
import { LucideMinus, LucidePlus } from 'lucide-react';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import { selectAccountInfoFromId } from '@/features/accounts/store/accountSlice';
import { type Transaction, type MonthGroupStats } from '../../types/transaction.types';
import { TransactionSkeleton } from '../TransactionSkeleton';
import { List } from 'react-window';
import { TransactionRow } from '../TransactionRow';
import { TransactionDetailPanel } from '../TransactionDetailPanel';
import { selectSelectedBudget } from '@/features/budget';
import { TransactionMobile } from '../TransactionMobile';
import { TransactionHeader, type MobileFilter } from '../TransactionHeader';
import {
  applyParsedInputValue,
  buildTransactionPayload,
  createEmptyTransaction,
  filterTransactions,
  getStickyHeaderForStartIndex,
  getTransactionColumns,
  getTransactionRowHeight,
  groupTransactions,
} from './transaction.helpers';

type StickyHeaderState = { label: string; stats: MonthGroupStats } | null;

function useIsMobile(breakpoint = 768) {
  const [isMobile, setIsMobile] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia(`(max-width: ${breakpoint}px)`).matches : false,
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

function StickyMonthHeader({ stickyHeader }: { stickyHeader: NonNullable<StickyHeaderState> }) {
  return (
    <div className={styles.stickyHeaderOverlay}>
      <span className={styles.monthGroupLabel}>{stickyHeader.label}</span>
      <span className={styles.monthGroupStats}>
        <span className={styles.statCount}>
          {stickyHeader.stats.count} transaction{stickyHeader.stats.count !== 1 ? 's' : ''}
        </span>
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
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export function Transaction() {
  const { setHeaderContent } = useHeader();
  const { setSidePanelContent } = useSidePanel();
  const { id } = useParams();
  const paramId = id ?? '';
  const dispatch = useAppDispatch();
  const { loading, transactions } = useAppSelector((state) => state.transactions);
  const { name: accountName, balance: accountBal } = useAppSelector((state) =>
    selectAccountInfoFromId(state, paramId ?? ''),
  );
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const selectedBudgetId = selectedBudget?.id ?? '';
  const [selectedTxn, setSelectedTxn] = useState<Transaction | null>(null);
  const [inlineEditingTxnId, setInlineEditingTxnId] = useState<string | null>(null);
  const [isDetailPanelOpen, setIsDetailPanelOpen] = useState(false);
  const [isAddingNew, setIsAddingNew] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [mobileFilter, setMobileFilter] = useState<MobileFilter>('all');
  const [stickyHeader, setStickyHeader] = useState<StickyHeaderState>(null);
  const cols = useMemo(() => getTransactionColumns(paramId), [paramId]);
  const isMobile = useIsMobile();

  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setSelectedTxn(null);
        setInlineEditingTxnId(null);
        setIsDetailPanelOpen(false);
        setIsAddingNew(false);
        setSidePanelContent(null);
      }
    };
    setSelectedTxn(null);
    setInlineEditingTxnId(null);
    setIsDetailPanelOpen(false);
    setIsAddingNew(false);
    setSidePanelContent(null);
    dispatch(fetchAllTransaction(paramId));
    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [paramId, dispatch, setSidePanelContent]);

  const handleTxnSelect = useCallback(
    (index: number, txn: Transaction | null) => {
      if (isAddingNew) return; // don't overwrite add-new mode
      setInlineEditingTxnId(null);
      setIsDetailPanelOpen(Boolean(txn));
      setIsAddingNew(false);
      setSelectedTxn(txn);
      void index; // originalIndex carried in ListItem, not needed here
    },
    [isAddingNew],
  );

  const handleInlineTxnEdit = useCallback(
    (index: number, txn: Transaction | null) => {
      if (isAddingNew) return;
      setInlineEditingTxnId(txn?.id ?? null);
      setIsAddingNew(false);
      setSelectedTxn(txn);
      void index;
    },
    [isAddingNew],
  );

  const resetSelectedTxn = useCallback(() => {
    setSelectedTxn(null);
    setInlineEditingTxnId(null);
    setIsDetailPanelOpen(false);
    setIsAddingNew(false);
    setSidePanelContent(null);
  }, [setSidePanelContent]);

  const handleSelectedTxnChange = useCallback((key: keyof Transaction, value: string | number | null) => {
    setSelectedTxn((prev) => {
      if (!prev) return null;
      return { ...prev, [key]: value };
    });
  }, []);

  const handleInputBlur = useCallback(
    (key: keyof Transaction, value: string | number) => {
      if (!selectedTxn || !value) return;
      try {
        const updatedTxn = applyParsedInputValue(selectedTxn, key, value);
        setSelectedTxn(updatedTxn);
        if (!isAddingNew && updatedTxn.id) {
          dispatch(updateTransaction(buildTransactionPayload(updatedTxn)))
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

  const handleAutoSave = useCallback(
    (overrides: Partial<Transaction>) => {
      if (!selectedTxn || isAddingNew || !selectedTxn.id) return;
      const updatedTxn: Transaction = { ...selectedTxn, ...overrides };
      dispatch(updateTransaction(buildTransactionPayload(updatedTxn)))
        .unwrap()
        .then(() => toast.success('Saved'))
        .catch(() => toast.error('Failed to save'));
    },
    [selectedTxn, isAddingNew, dispatch],
  );

  const addTransaction = useCallback(() => {
    setSelectedTxn(
      createEmptyTransaction({
        budgetId: selectedBudgetId,
        accountId: paramId,
        balance: transactions[0]?.balance ?? 0,
      }),
    );
    setIsDetailPanelOpen(true);
    setIsAddingNew(true);
  }, [selectedBudgetId, transactions, paramId]);

  const handleSave = useCallback(async () => {
    if (!selectedTxn) return;
    try {
      if (isAddingNew) {
        await dispatch(createTransaction(buildTransactionPayload(selectedTxn))).unwrap();
        toast.success('Transaction created');
        dispatch(fetchAllTransaction(paramId || ''));
      } else {
        await dispatch(updateTransaction(buildTransactionPayload(selectedTxn))).unwrap();
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

  const handleStatusChange = useCallback(
    async (status: 'APPROVED' | 'REJECTED') => {
      if (!selectedTxn?.id) return;
      try {
        await dispatch(updateTransactionStatus({ id: selectedTxn.id, status })).unwrap();
        dispatch(fetchAllTransaction(paramId ? selectedTxn.accountId : ''));
        toast.success(`Transaction ${status.toLowerCase()}`);
        resetSelectedTxn();
      } catch {
        toast.error('Failed to update status');
      }
    },
    [selectedTxn, dispatch, paramId, resetSelectedTxn],
  );

  const handlePanelSelectChange = useCallback(
    (idKey: keyof Transaction, nameKey: keyof Transaction) => (id: string, name: string) => {
      handleSelectedTxnChange(idKey, id);
      handleSelectedTxnChange(nameKey, name);
    },
    [handleSelectedTxnChange],
  );

  const filteredTransactions = useMemo(() => {
    return filterTransactions(transactions, searchTerm, mobileFilter);
  }, [transactions, searchTerm, mobileFilter]);

  const listItems = useMemo(() => groupTransactions(filteredTransactions), [filteredTransactions]);

  const rowProps = useMemo(
    () => ({
      paramId,
      listItems,
      cols,
      isAddingNew,
      selectedTxn,
      inlineEditingTxnId,
      handleTxnSelect,
      handleInlineTxnEdit,
      handleSelectedTxnChange,
      handleInputBlur,
      onAutoSave: handleAutoSave,
    }),
    [
      paramId,
      listItems,
      cols,
      isAddingNew,
      selectedTxn,
      inlineEditingTxnId,
      handleTxnSelect,
      handleInlineTxnEdit,
      handleSelectedTxnChange,
      handleInputBlur,
      handleAutoSave,
    ],
  );

  const dynamicRowHeight = useCallback(
    (index: number) => getTransactionRowHeight(isMobile, listItems[index]),
    [isMobile, listItems],
  );

  const handleRowsRendered = useCallback(
    ({ startIndex }: { startIndex: number }) => {
      setStickyHeader(getStickyHeaderForStartIndex(listItems, startIndex));
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

  // Sync detail panel into the side panel slot whenever selected txn changes
  useEffect(() => {
    if ((!selectedTxn || !isDetailPanelOpen) && !isAddingNew) {
      setSidePanelContent(null);
      return;
    }
    setSidePanelContent(
      <TransactionDetailPanel
        selectedTxn={selectedTxn}
        isAddingNew={isAddingNew}
        onChange={handleSelectedTxnChange}
        onSelectChange={handlePanelSelectChange}
        onSave={handleSave}
        onDelete={handleDelete}
        onClose={resetSelectedTxn}
        onStatusChange={handleStatusChange}
      />,
    );
  }, [
    selectedTxn,
    inlineEditingTxnId,
    isDetailPanelOpen,
    isAddingNew,
    setSidePanelContent,
    handleSelectedTxnChange,
    handlePanelSelectChange,
    handleSave,
    handleDelete,
    resetSelectedTxn,
    handleStatusChange,
  ]);

  return (
    <>
      {loading === LoadingState.PENDING && <TransactionSkeleton />}
      {loading === LoadingState.SUCCESS && (
        <div className={`${styles.wrapper} ${paramId ? styles.specificAccount : styles.allAccounts}`}>
          {isMobile ? (
            <TransactionMobile
              transactions={filteredTransactions}
              selectedTransactionId={selectedTxn?.id}
              showAccountName={!paramId}
              onSelectTransaction={handleTxnSelect}
            />
          ) : (
            <div className={styles.listArea}>
              <div className={styles.headerContainer}>
                {cols.map((col) => (
                  <div key={col.key} style={{ ...col.layout }} className={col.headerClassName?.join(' ')}>
                    {col.label}
                  </div>
                ))}
              </div>
              <div className={styles.txnContainer}>
                {stickyHeader && <StickyMonthHeader stickyHeader={stickyHeader} />}
                <List
                  className={styles.virtualList}
                  defaultHeight={500}
                  rowCount={listItems.length}
                  rowHeight={dynamicRowHeight}
                  rowComponent={TransactionRow}
                  rowProps={rowProps}
                  onRowsRendered={handleRowsRendered}
                />
              </div>
            </div>
          )}
        </div>
      )}
    </>
  );
}
