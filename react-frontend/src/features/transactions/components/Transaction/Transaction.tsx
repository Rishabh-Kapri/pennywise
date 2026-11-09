import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useLocation, useParams } from 'react-router-dom';
import { ArrowsLeftRightIcon } from '@phosphor-icons/react';
import {
  createTransaction,
  deleteTransactionById,
  fetchAllTransaction,
  updateTransaction,
  updateTransactionStatus,
} from '../../store';
import { LoadingState } from '@/utils';
import { errorMessage, toast } from '@/utils';
import styles from './Transaction.module.css';
import { useHeader } from '@/context/HeaderContext';
import { useSidePanel } from '@/context/SidePanelContext';
import { formatBudgetAmount } from '@/features/category/components/budgetFormat';
import { selectAccountInfoFromId } from '@/features/accounts/store/accountSlice';
import { BankLogo } from '@/features/accounts/components/BankLogo/BankLogo';
import {
  BudgetAccountNames,
  LoanAccountNames,
  TrackingAccountNames,
} from '@/features/accounts/types/account.types';
import {
  TransactionStatus,
  type Transaction,
  type MonthGroupStats,
  type TransactionStatus as TransactionStatusType,
} from '../../types/transaction.types';
import {
  EMPTY_FILTERS,
  hasActiveFilters,
  type TransactionFilters,
  type TransactionsLocationState,
} from '../../types/filter.types';
import { TransactionSkeleton } from '../TransactionSkeleton';
import { List } from 'react-window';
import { TransactionRow } from '../TransactionRow';
import { TransactionDetailPanel } from '../TransactionDetailPanel';
import { TransactionFilterPanel } from '../TransactionFilterPanel';
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
import { useDebounce } from '@/hooks/useDebounce';

const accountTypeNames = new Map<string, string>(
  [...BudgetAccountNames, ...TrackingAccountNames, ...LoanAccountNames].map(({ value, name }) => [value, name]),
);
const accountTypeName = (type?: string) => (type && accountTypeNames.get(type)) || 'Account';

/** "Swiggy · −₹450", the line under a transaction toast. */
function describeTxn(txn: Transaction) {
  const inflow = Number(txn.inflow) || 0;
  const outflow = Number(txn.outflow) || 0;
  const amount = outflow ? `−${formatBudgetAmount(outflow)}` : `+${formatBudgetAmount(inflow)}`;
  return `${txn.payeeName || 'No payee'} · ${amount}`;
}

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
          <span className={styles.statOutflow}>-{formatBudgetAmount(stickyHeader.stats.totalOutflow)}</span>
        )}
        {stickyHeader.stats.totalInflow > 0 && (
          <span className={styles.statInflow}>+{formatBudgetAmount(stickyHeader.stats.totalInflow)}</span>
        )}
      </span>
    </div>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export function Transaction({ accountId, embedded = false }: { accountId?: string; embedded?: boolean } = {}) {
  const { setHeaderContent } = useHeader();
  const { setSidePanelContent } = useSidePanel();
  const { id } = useParams();
  const paramId = accountId ?? id ?? '';
  const dispatch = useAppDispatch();
  const { loading, transactions, loadingMore, nextCursor, error } = useAppSelector((state) => state.transactions);
  const { name: accountName, balance: accountBal, type: accountType, bank: accountBank } = useAppSelector((state) =>
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
  // Other screens can open this one pre-filtered (Payees → "View transactions").
  // Built once: useDebounce re-applies its seed whenever the seed's identity changes.
  const location = useLocation();
  const [initialFilters] = useState<TransactionFilters>(() => {
    const payee = (location.state as TransactionsLocationState | null)?.payeeFilter;
    return payee ? { ...EMPTY_FILTERS, payeeIds: [payee.id], payeeNames: [payee.name] } : EMPTY_FILTERS;
  });
  const [filters, setFilters] = useDebounce<TransactionFilters>(initialFilters, 250);
  // The filter fields stay folded behind the toolbar's Filters button until asked for.
  const [filtersOpen, setFiltersOpen] = useState(() => hasActiveFilters(initialFilters));
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
    dispatch(fetchAllTransaction({ accountIds: paramId ? [paramId] : [] }));
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('keydown', handleEscape);
      setSidePanelContent(null);
    };
  }, [paramId, selectedBudgetId, dispatch, setSidePanelContent]);

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

  const handleSelectedTxnChange = useCallback((key: keyof Transaction, value: Transaction[keyof Transaction]) => {
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
        const optimisticTxn: Transaction = {
          ...updatedTxn,
          status: updatedTxn.status === TransactionStatus.UNAPPROVED ? TransactionStatus.APPROVED : updatedTxn.status,
        };
        setSelectedTxn(updatedTxn);
        if (!isAddingNew && updatedTxn.id) {
          dispatch(
            updateTransaction({
              payload: buildTransactionPayload(optimisticTxn),
              optimisticTransaction: optimisticTxn,
            }),
          )
            .unwrap()
            .then(() => toast.success('Changes saved', { description: describeTxn(optimisticTxn) }))
            .catch((err) =>
              toast.error("Couldn't save your change", { description: errorMessage(err, 'Please try again.') }),
            );
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
      const optimisticTxn: Transaction = {
        ...updatedTxn,
        status: updatedTxn.status === TransactionStatus.UNAPPROVED ? TransactionStatus.APPROVED : updatedTxn.status,
      };
      dispatch(
        updateTransaction({
          payload: buildTransactionPayload(optimisticTxn),
          optimisticTransaction: optimisticTxn,
        }),
      )
        .unwrap()
        .then(() => toast.success('Changes saved', { description: describeTxn(optimisticTxn) }))
        .catch((err) =>
          toast.error("Couldn't save your change", { description: errorMessage(err, 'Please try again.') }),
        );
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
    if (!selectedTxn.accountId) {
      toast.warning('Pick an account', { description: 'Every transaction belongs to an account.' });
      return;
    }
    try {
      if (isAddingNew) {
        const payload = buildTransactionPayload(selectedTxn);
        await dispatch(createTransaction(payload)).unwrap();
        toast.success('Transaction added', { description: describeTxn(selectedTxn) });
        const accountIds = paramId ? [paramId] : [];
        const params = {
          ...(accountIds.length && { accountIds: accountIds }),
        };
        dispatch(fetchAllTransaction(params));
      } else {
        const optimisticTxn: Transaction = {
          ...selectedTxn,
          status: selectedTxn.status === TransactionStatus.UNAPPROVED ? TransactionStatus.APPROVED : selectedTxn.status,
        };
        await dispatch(
          updateTransaction({
            payload: buildTransactionPayload(optimisticTxn),
            optimisticTransaction: optimisticTxn,
          }),
        ).unwrap();
        toast.success('Transaction updated', { description: describeTxn(selectedTxn) });
      }
      resetSelectedTxn();
    } catch (err) {
      toast.error(isAddingNew ? "Couldn't add the transaction" : "Couldn't update the transaction", {
        description: errorMessage(err, 'Please try again.'),
      });
    }
  }, [selectedTxn, isAddingNew, dispatch, paramId, resetSelectedTxn]);

  const handleDelete = useCallback(async () => {
    if (!selectedTxn?.id) return;
    try {
      await dispatch(deleteTransactionById(selectedTxn.id)).unwrap();
      const accountIds = paramId ? [paramId] : [];
      const params = {
        ...(accountIds.length && { accountIds: accountIds }),
      };
      dispatch(fetchAllTransaction(params));
      toast.success('Transaction deleted', { description: describeTxn(selectedTxn) });
      resetSelectedTxn();
    } catch (err) {
      toast.error("Couldn't delete the transaction", { description: errorMessage(err, 'Please try again.') });
    }
  }, [selectedTxn, dispatch, paramId, resetSelectedTxn]);

  const handleStatusChange = useCallback(
    async (
      status: Extract<TransactionStatusType, typeof TransactionStatus.APPROVED | typeof TransactionStatus.REJECTED>,
    ) => {
      if (!selectedTxn?.id) return;
      try {
        await dispatch(updateTransactionStatus({ id: selectedTxn.id, status })).unwrap();
        toast.success(status === TransactionStatus.APPROVED ? 'Transaction approved' : 'Transaction rejected', {
          description: describeTxn(selectedTxn),
        });
        resetSelectedTxn();
      } catch (err) {
        toast.error(
          status === TransactionStatus.APPROVED ? "Couldn't approve the transaction" : "Couldn't reject the transaction",
          { description: errorMessage(err, 'Please try again.') },
        );
      }
    },
    [selectedTxn, dispatch, resetSelectedTxn],
  );

  const handlePanelSelectChange = useCallback(
    (idKey: keyof Transaction, nameKey: keyof Transaction) => (id: string, name: string) => {
      const isClearingCategory = idKey === 'categoryId' && !id && !name;
      handleSelectedTxnChange(idKey, isClearingCategory ? null : id);
      handleSelectedTxnChange(nameKey, isClearingCategory ? null : name);
    },
    [handleSelectedTxnChange],
  );

  const filteredTransactions = useMemo(() => {
    const search = searchTerm.trim().toLowerCase();
    const scopedTransactions = transactions.filter((transaction) =>
      (!paramId || transaction.accountId === paramId) &&
      (!search || [transaction.payeeName, transaction.categoryName, transaction.note, transaction.accountName]
        .some((value) => value?.toLowerCase().includes(search))),
    );
    return filterTransactions(scopedTransactions, mobileFilter);
  }, [transactions, mobileFilter, paramId, searchTerm]);

  const listItems = useMemo(() => groupTransactions(filteredTransactions), [filteredTransactions]);
  const isLoadingMore = loadingMore === LoadingState.PENDING;

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

  const loadMoreTransactions = useCallback(() => {
    if (!nextCursor || loadingMore === LoadingState.PENDING || loading === LoadingState.PENDING) {
      return;
    }
    const { accountIds, categoryIds, payeeIds, tagIds, note, dateFrom, dateTo } = filters;

    const args = {
      accountIds: paramId ? [paramId] : accountIds,
      categoryIds,
      payeeIds,
      tagIds,
      note,
      startDate: dateFrom,
      endDate: dateTo,
    };

    const params = {
      ...args,
      cursor: nextCursor,
    };
    dispatch(fetchAllTransaction(params));
  }, [dispatch, filters, loading, loadingMore, nextCursor, paramId]);

  const handleRowsRendered = useCallback(
    ({ startIndex, stopIndex }: { startIndex: number; stopIndex: number }) => {
      setStickyHeader(getStickyHeaderForStartIndex(listItems, startIndex));
      if (stopIndex >= listItems.length - 3) {
        loadMoreTransactions();
      }
    },
    [listItems, loadMoreTransactions],
  );

  useEffect(() => {
    const { accountIds, categoryIds, payeeIds, tagIds, note, dateFrom, dateTo } = filters;
    const args = {
      accountIds: paramId ? [paramId] : accountIds,
      categoryIds,
      payeeIds,
      tagIds,
      note,
      startDate: dateFrom,
      endDate: dateTo,
    };
    const timer = setTimeout(() => dispatch(fetchAllTransaction(args)), 500);

    return () => {
      clearTimeout(timer);
    };
  }, [filters, dispatch, paramId, selectedBudgetId]);

  // The page draws its own title and toolbar (as Budget does), so the shared header slot stays empty.
  useEffect(() => {
    if (embedded) return;
    setHeaderContent(null);
  }, [setHeaderContent, embedded]);

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
    paramId,
    setSidePanelContent,
    handleSelectedTxnChange,
    handlePanelSelectChange,
    handleSave,
    handleDelete,
    resetSelectedTxn,
    handleStatusChange,
  ]);

  return (
    <div className={`${styles.page} ${embedded ? '' : styles.standalonePage}`}>
      {!embedded && (
        <header className={styles.pageTop}>
          <div className={styles.titleBlock}>
            {paramId ? (
              <BankLogo name={accountName} bank={accountBank} type={accountType} size="lg" />
            ) : (
              <span className={styles.titleIcon}>
                <ArrowsLeftRightIcon size={22} weight="bold" />
              </span>
            )}
            <div className={styles.titleText}>
              <span className={styles.titleEyebrow}>{paramId ? accountTypeName(accountType) : selectedBudget?.name || 'All accounts'}</span>
              <h1>{paramId ? accountName : 'Transactions'}</h1>
            </div>
          </div>
          <dl className={styles.pageStats}>
            <div>
              <dt>{paramId ? 'Balance' : 'Net balance'}</dt>
              <dd className={accountBal < 0 ? styles.negative : undefined}>{formatBudgetAmount(accountBal)}</dd>
            </div>
          </dl>
        </header>
      )}
      <div className={styles.toolbar}>
        <TransactionHeader
          name={accountName}
          balance={accountBal}
          onTxnAdd={addTransaction}
          searchTerm={searchTerm}
          onSearchChange={setSearchTerm}
          mobileFilter={mobileFilter}
          onMobileFilterChange={setMobileFilter}
          showSummary={false}
          filterToggle={{
            open: filtersOpen,
            active: hasActiveFilters(filters),
            onToggle: () => setFiltersOpen((open) => !open),
          }}
        />
        {filtersOpen && (
          <div className={styles.filterCard}>
            <TransactionFilterPanel filters={filters} onChange={setFilters} hideAccountFilter={Boolean(paramId)} />
          </div>
        )}
      </div>
      {loading === LoadingState.PENDING && transactions.length === 0 && <TransactionSkeleton />}
      {error && <div className={styles.errorBanner}>{error}</div>}
      {loading === LoadingState.SUCCESS && filteredTransactions.length === 0 && (
        transactions.length === 0 && !hasActiveFilters(filters) ? (
          <p className={styles.emptyMessage}>
            No transactions yet. Bank alert emails show up here on their own once an AI provider is connected in{' '}
            <Link to="/settings?section=ai">Settings → AI</Link>; you can also add one by hand.
          </p>
        ) : (
          <p className={styles.emptyMessage}>No transactions match this view.</p>
        )
      )}
      {(loading !== LoadingState.PENDING || transactions.length > 0) && (
        <div className={`${styles.wrapper} ${paramId ? styles.specificAccount : styles.allAccounts} ${styles.listWrapper}`}>
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
              <div className={`${styles.txnContainer} ${isLoadingMore ? styles.txnContainerLoadingMore : ''}`}>
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
                {isLoadingMore && (
                  <div
                    className={styles.bottomLoader}
                    role="status"
                    aria-live="polite"
                    aria-label="Loading more transactions">
                    <span className={styles.loaderSpinner} />
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      )}
      {isMobile && nextCursor && (
        <button type="button" onClick={loadMoreTransactions} disabled={isLoadingMore || loading === LoadingState.PENDING}>
          {isLoadingMore ? 'Loading…' : 'Load more transactions'}
        </button>
      )}
    </div>
  );
}
