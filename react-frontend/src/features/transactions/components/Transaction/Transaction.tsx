import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { ArrowsLeftRightIcon } from '@phosphor-icons/react';
import {
  createTransaction,
  deleteTransactionById,
  fetchAllTransaction,
  updateTransaction,
  updateTransactionStatus,
  bulkDeleteTransactions,
  fetchPendingApprovalCount,
} from '../../store';
import { LoadingState } from '@/utils';
import { apiClient, errorMessage, toast } from '@/utils';
import type { PaginationResponse } from '@/utils/common.constants';
import styles from './Transaction.module.css';
import { useHeader, useSidePanel } from '@/components/layout/slots';
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
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { runJobAction } from '@/features/jobs/store';
import { ActiveFilterChips, TransactionFilterPanel } from '../TransactionFilterPanel';
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
import { useFilteredTotals } from '../../hooks/useFilteredTotals';
import { useTransactionFacets } from '../../hooks/useTransactionFacets';

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

/** "+1,234 pts", or "−40 pts" when more points were taken back than earned. */
const formatPoints = (n: number) => `${n < 0 ? '−' : '+'}${Math.abs(n).toLocaleString('en-IN')} pts`;

export function Transaction({ accountId, embedded = false }: { accountId?: string; embedded?: boolean } = {}) {
  const { setHeaderContent } = useHeader();
  const { setSidePanelContent } = useSidePanel();
  // /transactions/:id is a transaction, whose details open; an account's list
  // is embedded in Accounts with accountId.
  const { id: routeId } = useParams();
  const paramId = accountId ?? '';
  // Only the Transactions page puts the open transaction in the URL.
  const syncUrl = !embedded && !accountId;
  const dispatch = useAppDispatch();
  const { loading, transactions, loadingMore, nextCursor, error, pendingApprovalCount } = useAppSelector(
    (state) => state.transactions,
  );
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
  const [bulkSelectedIds, setBulkSelectedIds] = useState<Set<string>>(() => new Set());
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [bulkApproving, setBulkApproving] = useState(false);
  // Other screens can open this one pre-filtered (Payees → "View transactions").
  // Built once: useDebounce re-applies its seed whenever the seed's identity changes.
  const location = useLocation();
  const navigate = useNavigate();
  // Set while the add form holds an email from Jobs, cleared if it's abandoned.
  const dismissJobIdRef = useRef<string | null>(null);
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
        dismissJobIdRef.current = null;
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

  // Runs after the reset above, so the form it opens stays open.
  useEffect(() => {
    const state = location.state as TransactionsLocationState | null;
    if (!state?.prefill) return;
    setSelectedTxn({
      ...createEmptyTransaction({ budgetId: selectedBudgetId, accountId: paramId, balance: 0 }),
      ...state.prefill,
    });
    setIsDetailPanelOpen(true);
    setIsAddingNew(true);
    dismissJobIdRef.current = state.dismissJobId ?? null;
    // Otherwise a reload or the back button would open the form again.
    navigate(location.pathname, { replace: true, state: null });
  }, [location.pathname, location.state, navigate, paramId, selectedBudgetId]);

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

  // ── The open transaction ↔ /transactions/:id ──────────────────────────────
  // Each side remembers the last value it saw, so a change made by the other
  // side (opening from a link, or the URL following a click) isn't acted on twice.
  const allAccounts = useAppSelector((state) => state.accounts.allAccounts);
  const accountsRef = useRef(allAccounts);
  accountsRef.current = allAccounts;
  const routeTxnId = syncUrl ? routeId : undefined;
  const openTxnId = isDetailPanelOpen && !isAddingNew ? (selectedTxn?.id ?? undefined) : undefined;
  const seenRoute = useRef<string | undefined>(undefined);
  const seenOpen = useRef<string | undefined>(undefined);
  const loadingRoute = useRef<string | undefined>(undefined);

  // URL → panel: a link (or Back/Forward) names a transaction to open.
  useEffect(() => {
    if (!syncUrl || routeTxnId === seenRoute.current) return;
    seenRoute.current = routeTxnId;
    if (!routeTxnId) {
      loadingRoute.current = undefined;
      if (openTxnId) {
        seenOpen.current = undefined;
        setSelectedTxn(null);
        setIsDetailPanelOpen(false);
      }
      return;
    }
    if (routeTxnId === openTxnId) return;

    // An old link to an account's transactions: that list lives in Accounts now.
    const toAccount = () => {
      if (!accountsRef.current.some((a) => a.id === routeTxnId)) return false;
      navigate(`/accounts?account=${routeTxnId}`, { replace: true });
      return true;
    };
    if (toAccount()) return;

    loadingRoute.current = routeTxnId;
    apiClient
      .get<PaginationResponse<Transaction[]>>(`transactions/normalized?id=${encodeURIComponent(routeTxnId)}&limit=1`)
      .then((res) => {
        if (loadingRoute.current !== routeTxnId) return; // the URL moved on
        const txn = res.data?.[0];
        if (!txn) {
          if (toAccount()) return;
          toast.error("Couldn't find that transaction", { description: 'It may have been deleted.' });
          navigate('/transactions', { replace: true });
          return;
        }
        loadingRoute.current = undefined;
        seenOpen.current = txn.id;
        setInlineEditingTxnId(null);
        setIsAddingNew(false);
        setSelectedTxn(txn);
        setIsDetailPanelOpen(true);
      })
      .catch((err) => {
        if (loadingRoute.current !== routeTxnId) return;
        toast.error("Couldn't open that transaction", { description: errorMessage(err, 'Try again.') });
      });
  }, [syncUrl, routeTxnId, openTxnId, navigate]);

  // Panel → URL: opening, switching and closing details update the address.
  useEffect(() => {
    if (!syncUrl || openTxnId === seenOpen.current) return;
    seenOpen.current = openTxnId;
    if (openTxnId && openTxnId !== routeTxnId) {
      seenRoute.current = openTxnId;
      // switching between transactions replaces, so Back leaves the page
      navigate(`/transactions/${openTxnId}`, { replace: Boolean(routeTxnId) });
    } else if (!openTxnId && routeTxnId && loadingRoute.current !== routeTxnId) {
      seenRoute.current = undefined;
      navigate('/transactions');
    }
  }, [syncUrl, openTxnId, routeTxnId, navigate]);

  const resetSelectedTxn = useCallback(() => {
    dismissJobIdRef.current = null;
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
    dismissJobIdRef.current = null;
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
        if (dismissJobIdRef.current) {
          // The transaction is saved either way; the email just stays in Jobs.
          dispatch(runJobAction({ id: dismissJobIdRef.current, action: 'dismiss' }));
        }
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
      // Drops a row as soon as it's approved, without reloading the list.
      (!filters.pendingOnly || transaction.status === TransactionStatus.UNAPPROVED) &&
      (!search || [transaction.payeeName, transaction.categoryName, transaction.note, transaction.accountName]
        .some((value) => value?.toLowerCase().includes(search))),
    );
    return filterTransactions(scopedTransactions, mobileFilter);
  }, [transactions, mobileFilter, paramId, searchTerm, filters.pendingOnly]);

  const listItems = useMemo(() => groupTransactions(filteredTransactions), [filteredTransactions]);

  // Only rows on screen count, so a delete can't reach one a search has hidden.
  const visibleIds = useMemo(
    () => filteredTransactions.flatMap((transaction) => (transaction.id ? [transaction.id] : [])),
    [filteredTransactions],
  );
  const bulkIds = useMemo(() => visibleIds.filter((id) => bulkSelectedIds.has(id)), [visibleIds, bulkSelectedIds]);
  const allBulkSelected = visibleIds.length > 0 && bulkIds.length === visibleIds.length;
  // Only unapproved rows can be approved; the rest of a selection is left as it is.
  const bulkApprovableIds = useMemo(
    () =>
      filteredTransactions.flatMap((transaction) =>
        transaction.id && bulkSelectedIds.has(transaction.id) && transaction.status === TransactionStatus.UNAPPROVED
          ? [transaction.id]
          : [],
      ),
    [filteredTransactions, bulkSelectedIds],
  );

  const handleBulkToggle = useCallback((id: string) => {
    setBulkSelectedIds((current) => {
      const next = new Set(current);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  }, []);

  useEffect(() => {
    setBulkSelectedIds(new Set());
  }, [filters, paramId, selectedBudgetId]);
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
      bulkSelectedIds,
      onBulkToggle: handleBulkToggle,
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
      bulkSelectedIds,
      handleBulkToggle,
    ],
  );

  const dynamicRowHeight = useCallback(
    (index: number) => getTransactionRowHeight(isMobile, listItems[index]),
    [isMobile, listItems],
  );

  const listArgs = useMemo(() => {
    const { accountIds, categoryIds, payeeIds, tagIds, note, dateFrom, dateTo, pendingOnly } = filters;
    return {
      accountIds: paramId ? [paramId] : accountIds,
      categoryIds,
      payeeIds,
      tagIds,
      note,
      startDate: dateFrom,
      endDate: dateTo,
      status: pendingOnly ? TransactionStatus.UNAPPROVED : undefined,
    };
  }, [filters, paramId]);

  const filtersActive = hasActiveFilters(filters);
  const filteredTotals = useFilteredTotals(listArgs, filtersActive && !embedded, transactions);
  // What the results hold, listed first in the tray's payee, account and tag menus.
  const facets = useTransactionFacets(listArgs, filtersOpen, transactions);
  const filteredNet = filteredTotals ? filteredTotals.inflow - filteredTotals.outflow : 0;

  const loadMoreTransactions = useCallback(() => {
    if (!nextCursor || loadingMore === LoadingState.PENDING || loading === LoadingState.PENDING) {
      return;
    }
    dispatch(fetchAllTransaction({ ...listArgs, cursor: nextCursor }));
  }, [dispatch, listArgs, loading, loadingMore, nextCursor]);

  const handleBulkDelete = useCallback(async () => {
    const count = bulkIds.length;
    const label = `${count} transaction${count === 1 ? '' : 's'}`;
    if (!window.confirm(`Delete ${label}? A transfer's other side is deleted with it.`)) return;
    setBulkDeleting(true);
    try {
      await dispatch(bulkDeleteTransactions(bulkIds)).unwrap();
      toast.success(`${label} deleted`);
      if (selectedTxn?.id && bulkIds.includes(selectedTxn.id)) resetSelectedTxn();
      setBulkSelectedIds(new Set());
      dispatch(fetchAllTransaction(listArgs));
      dispatch(fetchAllAccounts());
    } catch (err) {
      toast.error(`Couldn't delete ${label}`, { description: errorMessage(err, 'Nothing was deleted. Please try again.') });
    } finally {
      setBulkDeleting(false);
    }
  }, [bulkIds, dispatch, listArgs, resetSelectedTxn, selectedTxn?.id]);

  const handleBulkApprove = useCallback(async () => {
    const ids = bulkApprovableIds;
    const label = `${ids.length} transaction${ids.length === 1 ? '' : 's'}`;
    setBulkApproving(true);
    // The status route learns from each approval and moves a transfer's other half,
    // so approve through it, a few at a time, rather than around it.
    const failed: string[] = [];
    let lastError: unknown;
    const queue = [...ids];
    const worker = async () => {
      for (let id = queue.shift(); id; id = queue.shift()) {
        try {
          await dispatch(updateTransactionStatus({ id, status: TransactionStatus.APPROVED })).unwrap();
        } catch (err) {
          failed.push(id);
          lastError = err;
        }
      }
    };
    try {
      await Promise.all(Array.from({ length: Math.min(4, ids.length) }, worker));
      const approved = ids.length - failed.length;
      if (failed.length === 0) {
        toast.success(`${label} approved`);
      } else {
        toast.error(`Couldn't approve ${failed.length} of ${label}`, {
          description: errorMessage(lastError, 'The rest were approved. Please try again.'),
        });
      }
      if (approved > 0 && selectedTxn?.id && ids.includes(selectedTxn.id) && !failed.includes(selectedTxn.id)) {
        resetSelectedTxn();
      }
      // Keep the rows that failed selected so a retry is one click.
      setBulkSelectedIds(new Set(failed));
    } finally {
      setBulkApproving(false);
    }
  }, [bulkApprovableIds, dispatch, resetSelectedTxn, selectedTxn?.id]);

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
    const timer = setTimeout(() => dispatch(fetchAllTransaction(listArgs)), 500);

    return () => {
      clearTimeout(timer);
    };
  }, [listArgs, dispatch, selectedBudgetId]);

  // Every reload and edit, including websocket updates, ends by setting loading to SUCCESS.
  useEffect(() => {
    if (loading === LoadingState.SUCCESS) dispatch(fetchPendingApprovalCount(paramId));
  }, [dispatch, loading, paramId, selectedBudgetId]);

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
            {filtersActive ? (
              <div
                className={styles.filteredStat}
                tabIndex={0}
                aria-describedby="filtered-total-breakdown">
                <dt>Filtered total</dt>
                <dd className={filteredNet < 0 ? styles.negative : undefined}>
                  {filteredTotals ? formatBudgetAmount(filteredNet) : '…'}
                </dd>
                {filteredTotals?.rewardPoints !== undefined && (
                  <dd className={styles.pointsStat}>{formatPoints(filteredTotals.rewardPoints)}</dd>
                )}
                {filteredTotals && (
                  <div id="filtered-total-breakdown" role="tooltip" className={styles.totalsTooltip}>
                    <div className={styles.totalsLine}>
                      <span>Inflow</span>
                      <span className={styles.inflow}>+{formatBudgetAmount(filteredTotals.inflow)}</span>
                    </div>
                    <div className={styles.totalsLine}>
                      <span>Outflow</span>
                      <span className={styles.outflow}>−{formatBudgetAmount(filteredTotals.outflow)}</span>
                    </div>
                    <div className={`${styles.totalsLine} ${styles.totalsNet}`}>
                      <span>Net</span>
                      <span>{formatBudgetAmount(filteredNet)}</span>
                    </div>
                    {filteredTotals.rewardPoints !== undefined && (
                      <div className={`${styles.totalsLine} ${styles.totalsPoints}`}>
                        <span>Reward points</span>
                        <span>{formatPoints(filteredTotals.rewardPoints)}</span>
                      </div>
                    )}
                    <p className={styles.totalsCount}>
                      {filteredTotals.count} {filteredTotals.count === 1 ? 'transaction' : 'transactions'}
                      {filteredTotals.rewardPoints !== undefined &&
                        ` · points from the ${filteredTotals.rewardPointsCount} on a card statement`}
                    </p>
                  </div>
                )}
              </div>
            ) : (
              <div>
                <dt>{paramId ? 'Balance' : 'Net balance'}</dt>
                <dd className={accountBal < 0 ? styles.negative : undefined}>{formatBudgetAmount(accountBal)}</dd>
              </div>
            )}
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
          pendingApproval={{
            count: pendingApprovalCount,
            active: filters.pendingOnly,
            onToggle: () => setFilters((current) => ({ ...current, pendingOnly: !current.pendingOnly })),
          }}
          filterToggle={{
            open: filtersOpen,
            active: hasActiveFilters(filters),
            onToggle: () => setFiltersOpen((open) => !open),
          }}
        />
        {filtersOpen && (
          <div className={styles.filterCard}>
            <TransactionFilterPanel
              filters={filters}
              onChange={setFilters}
              hideAccountFilter={Boolean(paramId)}
              facets={facets}
            />
          </div>
        )}
        <ActiveFilterChips filters={filters} onChange={setFilters} hideAccounts={Boolean(paramId)} />
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
                <label className={styles.bulkCell}>
                  <input
                    type="checkbox"
                    className={styles.bulkCheckbox}
                    aria-label="Select all loaded transactions"
                    checked={allBulkSelected}
                    disabled={visibleIds.length === 0}
                    ref={(input) => {
                      if (input) input.indeterminate = bulkIds.length > 0 && !allBulkSelected;
                    }}
                    onChange={() => setBulkSelectedIds(new Set(allBulkSelected ? [] : visibleIds))}
                  />
                </label>
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
              {bulkIds.length > 0 && (
                <div className={styles.bulkBar} role="toolbar" aria-label="Selected transactions">
                  <span className={styles.bulkCount}>{bulkIds.length} selected</span>
                  <button
                    type="button"
                    className={styles.bulkButton}
                    disabled={bulkDeleting || bulkApproving}
                    onClick={() => setBulkSelectedIds(new Set())}>
                    Clear
                  </button>
                  <button
                    type="button"
                    className={`${styles.bulkButton} ${styles.bulkApprove}`}
                    disabled={bulkDeleting || bulkApproving || bulkApprovableIds.length === 0}
                    title={bulkApprovableIds.length === 0 ? 'None of the selected transactions need approval' : undefined}
                    onClick={() => void handleBulkApprove()}>
                    {bulkApproving ? 'Approving…' : `Approve${bulkApprovableIds.length > 0 ? ` (${bulkApprovableIds.length})` : ''}`}
                  </button>
                  <button
                    type="button"
                    className={`${styles.bulkButton} ${styles.bulkDelete}`}
                    disabled={bulkDeleting || bulkApproving}
                    onClick={() => void handleBulkDelete()}>
                    {bulkDeleting ? 'Deleting…' : 'Delete'}
                  </button>
                </div>
              )}
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
