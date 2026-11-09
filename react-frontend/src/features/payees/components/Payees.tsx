import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useHeader } from '@/context/HeaderContext';
import { selectInflowCategory } from '@/features/category/store/categorySlice';
import {
  createPayee,
  deletePayeeRule,
  deletePayees,
  mergePayees,
  type PayeeRuleInput,
  refreshPayeeData,
  renamePayee,
  savePayeeRule,
} from '@/features/payees/store';
import type { PayeeRule } from '@/features/payees/types/payee.types';
import type { TransactionsLocationState } from '@/features/transactions/types/filter.types';
import { LoadingState, errorMessage, toast } from '@/utils';
import { ListChecks, MagnifyingGlass, Plus, Users, X } from '@phosphor-icons/react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { findDuplicateGroups } from '../payeeUtils';
import { MergePayeesModal } from './MergePayeesModal';
import { ConfirmDeleteDialog, NewPayeeDialog } from './PayeeDialogs';
import { PayeeDetail } from './PayeeDetail';
import { type PayeeFilter, PayeeList, type PayeeSort } from './PayeeList';
import { RulesView } from './RulesView';
import styles from './Payees.module.css';

type View = 'payees' | 'rules';

type PendingDelete = { kind: 'payees'; ids: string[] } | { kind: 'rule'; rule: PayeeRule };

export default function Payees() {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const { setHeaderContent } = useHeader();
  const { allPayees, loading, error, statsById, rules, rulesLoading } = useAppSelector((state) => state.payees);
  const categoryGroups = useAppSelector((state) => state.categories.allCategoryGroups);
  const inflowCategory = useAppSelector(selectInflowCategory);

  // view and selection live in the URL so back/forward and links work
  const [searchParams, setSearchParams] = useSearchParams();
  const view: View = searchParams.get('view') === 'rules' ? 'rules' : 'payees';
  const selectedId = searchParams.get('payee') ?? '';

  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<PayeeFilter>('all');
  const [sort, setSort] = useState<PayeeSort>('name');
  const [checkedIds, setCheckedIds] = useState<string[]>([]);
  const [merge, setMerge] = useState<{ ids: string[]; suggestedTargetId?: string } | null>(null);
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [saving, setSaving] = useState(false);
  const searchRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    setHeaderContent(null);
    dispatch(refreshPayeeData());
  }, [dispatch, setHeaderContent]);

  // "/" jumps to search from anywhere on the page
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== '/' || event.metaKey || event.ctrlKey) return;
      const target = event.target as HTMLElement;
      if (target.closest('input, textarea, select, [contenteditable="true"]')) return;
      event.preventDefault();
      searchRef.current?.focus();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, []);

  const setParams = useCallback(
    (next: { view?: View; payee?: string | null }) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          if (next.view) {
            if (next.view === 'rules') params.set('view', 'rules');
            else params.delete('view');
          }
          if (next.payee !== undefined) {
            if (next.payee) params.set('payee', next.payee);
            else params.delete('payee');
          }
          return params;
        },
        { replace: next.view === undefined },
      );
    },
    [setSearchParams],
  );

  const categoryNameById = useMemo(() => {
    const names: Record<string, string> = {};
    for (const group of categoryGroups) {
      for (const category of group.categories ?? []) if (category.id) names[category.id] = category.name;
    }
    if (inflowCategory?.id) names[inflowCategory.id] = inflowCategory.name;
    return names;
  }, [categoryGroups, inflowCategory]);

  const rulesByPayee = useMemo(() => {
    const grouped: Record<string, PayeeRule[]> = {};
    for (const rule of rules) (grouped[rule.payeeId] ??= []).push(rule);
    for (const list of Object.values(grouped)) list.sort((a, b) => a.matchString.localeCompare(b.matchString));
    return grouped;
  }, [rules]);

  const duplicateGroups = useMemo(() => findDuplicateGroups(allPayees, statsById), [allPayees, statsById]);
  const selectedPayee = allPayees.find((payee) => payee.id === selectedId) ?? null;
  const regularCount = allPayees.filter((payee) => !payee.transferAccountId).length;
  const unusedCount = allPayees.filter(
    (payee) => !payee.transferAccountId && payee.id && !(statsById[payee.id]?.transactionCount ?? 0),
  ).length;

  // drop checks for payees that no longer exist (merged or deleted)
  useEffect(() => {
    setCheckedIds((prev) => {
      const next = prev.filter((id) => allPayees.some((payee) => payee.id === id));
      return next.length === prev.length ? prev : next;
    });
  }, [allPayees]);

  // ── Actions ────────────────────────────────────────────────────────────────

  const run = async <T,>(task: () => Promise<T>, failTitle: string): Promise<T | undefined> => {
    setSaving(true);
    try {
      return await task();
    } catch (err) {
      toast.error(failTitle, { description: errorMessage(err, 'Please try again.') });
      return undefined;
    } finally {
      setSaving(false);
    }
  };

  const handleCreate = async (name: string) => {
    const created = await run(() => dispatch(createPayee(name)).unwrap(), "Couldn't add payee");
    if (!created) return;
    setIsCreating(false);
    setFilter('all');
    setParams({ view: 'payees', payee: created.id ?? null });
    toast.success('Payee added', { description: 'Add a rule so bank emails find it.' });
  };

  const handleRename = async (name: string) => {
    if (!selectedId) return false;
    const done = await run(async () => {
      await dispatch(renamePayee({ id: selectedId, name })).unwrap();
      return true;
    }, "Couldn't rename payee");
    if (done) toast.success('Payee renamed');
    return Boolean(done);
  };

  const handleSaveRule = async (input: PayeeRuleInput) => {
    const done = await run(async () => {
      await dispatch(savePayeeRule(input)).unwrap();
      return true;
    }, "Couldn't save rule");
    if (done) {
      const moved = input.originalPayeeId && input.originalPayeeId !== input.payeeId;
      toast.success(input.ruleId ? (moved ? 'Rule moved' : 'Rule updated') : 'Rule added');
    }
    return Boolean(done);
  };

  const handleMerge = async (targetId: string, sourceIds: string[], name: string) => {
    const target = allPayees.find((payee) => payee.id === targetId);
    const result = await run(async () => {
      const merged = await dispatch(mergePayees({ targetId, sourceIds })).unwrap();
      if (target && name && name !== target.name) await dispatch(renamePayee({ id: targetId, name })).unwrap();
      return merged;
    }, "Couldn't merge payees");
    if (!result) return;
    setMerge(null);
    setCheckedIds([]);
    setParams({ view: 'payees', payee: targetId });
    if (filter === 'duplicates' && duplicateGroups.length <= 1) setFilter('all');
    toast.success(`Merged ${result.mergedPayees + 1} payees`, {
      description: `${result.transactions} ${result.transactions === 1 ? 'transaction' : 'transactions'} and ${result.rules} ${result.rules === 1 ? 'rule' : 'rules'} moved to ${name || target?.name}.`,
    });
  };

  const confirmDelete = async () => {
    if (!pendingDelete) return;
    if (pendingDelete.kind === 'rule') {
      const { rule } = pendingDelete;
      const done = await run(async () => {
        await dispatch(deletePayeeRule({ payeeId: rule.payeeId, ruleId: rule.id })).unwrap();
        return true;
      }, "Couldn't delete rule");
      if (done) {
        setPendingDelete(null);
        toast.success('Rule deleted');
      }
      return;
    }
    const { ids } = pendingDelete;
    const failed = await run(() => dispatch(deletePayees(ids)).unwrap(), "Couldn't delete payees");
    if (!failed) return;
    setPendingDelete(null);
    setCheckedIds((prev) => prev.filter((id) => failed.includes(id)));
    if (ids.includes(selectedId) && !failed.includes(selectedId)) setParams({ payee: null });
    const deleted = ids.length - failed.length;
    if (failed.length === 0) toast.success(deleted === 1 ? 'Payee deleted' : `${deleted} payees deleted`);
    else toast.error(`Deleted ${deleted} of ${ids.length} payees`, { description: 'The rest are still selected; try again.' });
  };

  const viewTransactions = () => {
    if (!selectedPayee?.id) return;
    const state: TransactionsLocationState = { payeeFilter: { id: selectedPayee.id, name: selectedPayee.name } };
    navigate('/transactions', { state });
  };

  // ── Delete dialog copy ────────────────────────────────────────────────────

  const deleteDialog = (() => {
    if (!pendingDelete) return null;
    if (pendingDelete.kind === 'rule') {
      return {
        title: 'Delete this rule?',
        body: (
          <p>
            Bank strings matching <code>{pendingDelete.rule.matchString}</code> will no longer go straight to{' '}
            {pendingDelete.rule.payeeName ?? 'this payee'}. Existing transactions don’t change.
          </p>
        ),
        confirmLabel: 'Delete rule',
      };
    }
    const targets = pendingDelete.ids
      .map((id) => allPayees.find((payee) => payee.id === id))
      .filter((payee) => payee !== undefined);
    const txns = targets.reduce((sum, payee) => sum + (statsById[payee.id!]?.transactionCount ?? 0), 0);
    const ruleCount = targets.reduce((sum, payee) => sum + (statsById[payee.id!]?.ruleCount ?? 0), 0);
    const single = targets.length === 1 ? targets[0] : undefined;
    return {
      title: single ? `Delete ${single.name}?` : `Delete ${targets.length} payees?`,
      body: (
        <>
          {txns > 0 ? (
            <p>
              {txns} {txns === 1 ? 'transaction keeps' : 'transactions keep'} the name, but it can’t be picked again.
              Merging moves them to another payee instead.
            </p>
          ) : (
            <p>{single ? 'It has' : 'They have'} no transactions.</p>
          )}
          {ruleCount > 0 && (
            <p>
              {ruleCount} {ruleCount === 1 ? 'rule stops' : 'rules stop'} matching bank emails.
            </p>
          )}
        </>
      ),
      confirmLabel: single ? 'Delete payee' : `Delete ${targets.length} payees`,
      alternative:
        txns > 0
          ? {
              label: 'Merge instead',
              onClick: () => {
                setPendingDelete(null);
                setMerge({ ids: pendingDelete.ids });
              },
            }
          : undefined,
    };
  })();

  return (
    <section className={`${styles.page} ${selectedPayee && view === 'payees' ? styles.hasSelection : ''}`}>
      <header className={styles.top}>
        <div className={styles.titleBlock}>
          <h1>Payees</h1>
          <p>Who you pay, and the rules that recognise them in bank emails.</p>
        </div>
        <div className={styles.topActions}>
          <dl className={styles.stats}>
            <div>
              <dt>Payees</dt>
              <dd>{regularCount}</dd>
            </div>
            <div>
              <dt>Rules</dt>
              <dd>{rules.length}</dd>
            </div>
            <div>
              <dt>Unused</dt>
              <dd>{unusedCount}</dd>
            </div>
          </dl>
          <button type="button" className={styles.addButton} onClick={() => setIsCreating(true)}>
            <Plus size={16} weight="bold" /> New payee
          </button>
        </div>
      </header>

      <div className={styles.toolbar}>
        <div className={styles.tabs} role="tablist" aria-label="Payees view">
          <button
            type="button"
            role="tab"
            aria-selected={view === 'payees'}
            className={view === 'payees' ? styles.tabActive : ''}
            onClick={() => setParams({ view: 'payees' })}>
            <Users size={16} /> Payees
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={view === 'rules'}
            className={view === 'rules' ? styles.tabActive : ''}
            onClick={() => setParams({ view: 'rules' })}>
            <ListChecks size={16} /> Rules
          </button>
        </div>
        <label className={styles.search}>
          <MagnifyingGlass size={16} />
          <input
            ref={searchRef}
            value={search}
            placeholder={view === 'rules' ? 'Search bank strings, payees, categories' : 'Search payees and their bank strings'}
            onChange={(event) => setSearch(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                setSearch('');
                event.currentTarget.blur();
              }
            }}
          />
          {search ? (
            <button type="button" className={styles.searchClear} aria-label="Clear search" onClick={() => setSearch('')}>
              <X size={14} />
            </button>
          ) : (
            <kbd>/</kbd>
          )}
        </label>
      </div>

      {error && allPayees.length === 0 && <div className={styles.errorState}>{error}</div>}

      {view === 'payees' ? (
        <div className={styles.contentGrid}>
          <PayeeList
            payees={allPayees}
            statsById={statsById}
            rulesByPayee={rulesByPayee}
            duplicateGroups={duplicateGroups}
            search={search}
            filter={filter}
            sort={sort}
            selectedId={selectedId}
            checkedIds={checkedIds}
            loading={loading === LoadingState.PENDING}
            onFilterChange={(next) => {
              setFilter(next);
              setCheckedIds([]);
            }}
            onSortChange={setSort}
            onSelect={(id) => setParams({ payee: id })}
            onCheckedChange={setCheckedIds}
            onMerge={(ids, suggestedTargetId) => setMerge({ ids, suggestedTargetId })}
            onDelete={(ids) => setPendingDelete({ kind: 'payees', ids })}
          />
          <aside className={styles.detailPanel}>
            {selectedPayee ? (
              <PayeeDetail
                payee={selectedPayee}
                stats={statsById[selectedPayee.id!]}
                rules={rulesByPayee[selectedPayee.id!] ?? []}
                allRules={rules}
                payees={allPayees}
                categoryNameById={categoryNameById}
                saving={saving}
                onBack={() => setParams({ payee: null })}
                onRename={handleRename}
                onDelete={() => setPendingDelete({ kind: 'payees', ids: [selectedPayee.id!] })}
                onMerge={() => setMerge({ ids: [selectedPayee.id!], suggestedTargetId: selectedPayee.id })}
                onViewTransactions={viewTransactions}
                onSaveRule={handleSaveRule}
                onDeleteRule={(rule) => setPendingDelete({ kind: 'rule', rule })}
              />
            ) : (
              <div className={styles.emptyDetail}>
                <Users size={28} />
                <h3>Pick a payee</h3>
                <p>See how much goes to them, which category they usually land in, and the rules that match them.</p>
                {duplicateGroups.length > 0 && (
                  <button type="button" className={styles.secondaryButton} onClick={() => setFilter('duplicates')}>
                    Review {duplicateGroups.length} possible {duplicateGroups.length === 1 ? 'duplicate' : 'duplicates'}
                  </button>
                )}
                <p className={styles.kbdHint}>
                  <kbd>/</kbd> search · <kbd>↑</kbd>
                  <kbd>↓</kbd> move through the list
                </p>
              </div>
            )}
          </aside>
        </div>
      ) : (
        <RulesView
          rules={rules}
          payees={allPayees}
          categoryNameById={categoryNameById}
          search={search}
          loading={rulesLoading === LoadingState.PENDING}
          saving={saving}
          onOpenPayee={(id) => {
            setFilter('all');
            setSearch('');
            setParams({ view: 'payees', payee: id });
          }}
          onSaveRule={handleSaveRule}
          onDeleteRule={(rule) => setPendingDelete({ kind: 'rule', rule })}
        />
      )}

      <MergePayeesModal
        isOpen={merge !== null}
        initialIds={merge?.ids ?? []}
        suggestedTargetId={merge?.suggestedTargetId}
        payees={allPayees}
        statsById={statsById}
        saving={saving}
        onClose={() => setMerge(null)}
        onConfirm={handleMerge}
      />

      <ConfirmDeleteDialog
        isOpen={deleteDialog !== null}
        title={deleteDialog?.title ?? ''}
        body={deleteDialog?.body}
        confirmLabel={deleteDialog?.confirmLabel ?? 'Delete'}
        alternative={deleteDialog && 'alternative' in deleteDialog ? deleteDialog.alternative : undefined}
        saving={saving}
        onClose={() => setPendingDelete(null)}
        onConfirm={confirmDelete}
      />

      <NewPayeeDialog
        isOpen={isCreating}
        payees={allPayees}
        saving={saving}
        onClose={() => setIsCreating(false)}
        onCreate={handleCreate}
        onOpenExisting={(id) => {
          setIsCreating(false);
          setFilter('all');
          setParams({ view: 'payees', payee: id });
        }}
      />
    </section>
  );
}
