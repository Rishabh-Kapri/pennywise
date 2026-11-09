import { type KeyboardEvent, useEffect, useMemo, useRef } from 'react';
import { CaretRight, GitMerge, ListChecks, Sparkle, Trash, X } from '@phosphor-icons/react';
import { formatBudgetAmount } from '@/features/category/components/budgetFormat';
import type { Payee, PayeeRule, PayeeStats } from '../types/payee.types';
import { type DuplicateGroup, formatLastUsed } from '../payeeUtils';
import { PayeeAvatar } from './PayeeAvatar';
import styles from './Payees.module.css';

export type PayeeFilter = 'all' | 'rules' | 'norules' | 'unused' | 'duplicates' | 'transfers';
export type PayeeSort = 'name' | 'used' | 'recent' | 'spend';

const FILTERS: { id: PayeeFilter; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'rules', label: 'With rules' },
  { id: 'norules', label: 'No rules' },
  { id: 'unused', label: 'Unused' },
  { id: 'duplicates', label: 'Duplicates' },
  { id: 'transfers', label: 'Transfers' },
];

interface PayeeListProps {
  payees: Payee[];
  statsById: Record<string, PayeeStats>;
  rulesByPayee: Record<string, PayeeRule[]>;
  duplicateGroups: DuplicateGroup[];
  search: string;
  filter: PayeeFilter;
  sort: PayeeSort;
  selectedId: string;
  checkedIds: string[];
  loading: boolean;
  onFilterChange: (filter: PayeeFilter) => void;
  onSortChange: (sort: PayeeSort) => void;
  onSelect: (id: string) => void;
  onCheckedChange: (ids: string[]) => void;
  onMerge: (ids: string[], suggestedTargetId?: string) => void;
  onDelete: (ids: string[]) => void;
}

interface Row {
  payee: Payee;
  /** a rule string that matched the search when the name didn't */
  ruleHit?: string;
}

function matchSearch(payee: Payee, rules: PayeeRule[], query: string): Row | null {
  if (!query) return { payee };
  if (payee.name.toLowerCase().includes(query)) return { payee };
  const hit = rules.find(
    (rule) => rule.matchString.toLowerCase().includes(query) || rule.categoryName?.toLowerCase().includes(query),
  );
  return hit ? { payee, ruleHit: hit.matchString } : null;
}

function Highlight({ text, query }: { text: string; query: string }) {
  const index = query ? text.toLowerCase().indexOf(query) : -1;
  if (index < 0) return <>{text}</>;
  return (
    <>
      {text.slice(0, index)}
      <mark>{text.slice(index, index + query.length)}</mark>
      {text.slice(index + query.length)}
    </>
  );
}

export function PayeeList({
  payees,
  statsById,
  rulesByPayee,
  duplicateGroups,
  search,
  filter,
  sort,
  selectedId,
  checkedIds,
  loading,
  onFilterChange,
  onSortChange,
  onSelect,
  onCheckedChange,
  onMerge,
  onDelete,
}: PayeeListProps) {
  const listRef = useRef<HTMLDivElement | null>(null);
  const query = search.trim().toLowerCase();

  const counts = useMemo(() => {
    const regular = payees.filter((payee) => !payee.transferAccountId);
    return {
      all: regular.length,
      rules: regular.filter((payee) => (statsById[payee.id!]?.ruleCount ?? 0) > 0).length,
      norules: regular.filter((payee) => !(statsById[payee.id!]?.ruleCount ?? 0)).length,
      unused: regular.filter((payee) => !(statsById[payee.id!]?.transactionCount ?? 0)).length,
      duplicates: duplicateGroups.reduce((sum, group) => sum + group.payees.length, 0),
      transfers: payees.length - regular.length,
    } satisfies Record<PayeeFilter, number>;
  }, [payees, statsById, duplicateGroups]);

  const rows = useMemo(() => {
    const stat = (payee: Payee) => statsById[payee.id!];
    const keep = (payee: Payee) => {
      switch (filter) {
        case 'transfers':
          return Boolean(payee.transferAccountId);
        case 'rules':
          return !payee.transferAccountId && (stat(payee)?.ruleCount ?? 0) > 0;
        case 'norules':
          return !payee.transferAccountId && !(stat(payee)?.ruleCount ?? 0);
        case 'unused':
          return !payee.transferAccountId && !(stat(payee)?.transactionCount ?? 0);
        default:
          return !payee.transferAccountId;
      }
    };
    const compare = (a: Payee, b: Payee) => {
      const sa = stat(a);
      const sb = stat(b);
      switch (sort) {
        case 'used':
          return (sb?.transactionCount ?? 0) - (sa?.transactionCount ?? 0) || a.name.localeCompare(b.name);
        case 'recent':
          return (sb?.lastUsed ?? '').localeCompare(sa?.lastUsed ?? '') || a.name.localeCompare(b.name);
        case 'spend':
          return (sb?.totalOutflow ?? 0) - (sa?.totalOutflow ?? 0) || a.name.localeCompare(b.name);
        default:
          return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
      }
    };
    return payees
      .filter((payee) => payee.id && keep(payee))
      .sort(compare)
      .map((payee) => matchSearch(payee, rulesByPayee[payee.id!] ?? [], query))
      .filter((row): row is Row => row !== null);
  }, [payees, statsById, rulesByPayee, filter, sort, query]);

  const visibleGroups = useMemo(() => {
    if (filter !== 'duplicates') return [];
    return duplicateGroups
      .map((group) => ({
        ...group,
        rows: group.payees
          .map((payee) => matchSearch(payee, rulesByPayee[payee.id!] ?? [], query))
          .filter((row): row is Row => row !== null),
      }))
      .filter((group) => group.rows.length > 0);
  }, [filter, duplicateGroups, rulesByPayee, query]);

  const flatRows = filter === 'duplicates' ? visibleGroups.flatMap((group) => group.rows) : rows;
  const checkable = flatRows.filter((row) => !row.payee.transferAccountId).map((row) => row.payee.id!);
  const allChecked = checkable.length > 0 && checkable.every((id) => checkedIds.includes(id));

  useEffect(() => {
    listRef.current?.querySelector('[data-selected="true"]')?.scrollIntoView({ block: 'nearest' });
  }, [selectedId]);

  const toggle = (id: string) =>
    onCheckedChange(checkedIds.includes(id) ? checkedIds.filter((other) => other !== id) : [...checkedIds, id]);

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    if ((event.target as HTMLElement).tagName === 'INPUT') return;
    event.preventDefault();
    const index = flatRows.findIndex((row) => row.payee.id === selectedId);
    const next = flatRows[Math.min(Math.max(index + (event.key === 'ArrowDown' ? 1 : -1), 0), flatRows.length - 1)];
    if (next) onSelect(next.payee.id!);
  };

  const renderRow = ({ payee, ruleHit }: Row) => {
    const id = payee.id!;
    const stats = statsById[id];
    const isTransfer = Boolean(payee.transferAccountId);
    const isChecked = checkedIds.includes(id);
    const lastUsed = formatLastUsed(stats?.lastUsed);
    const count = stats?.transactionCount ?? 0;
    return (
      <div
        key={id}
        role="option"
        aria-selected={id === selectedId}
        data-selected={id === selectedId}
        className={`${styles.payeeRow} ${id === selectedId ? styles.payeeRowSelected : ''} ${isChecked ? styles.payeeRowChecked : ''}`}
        onClick={() => onSelect(id)}>
        {isTransfer ? (
          <span className={styles.checkSpacer} />
        ) : (
          <input
            type="checkbox"
            className={`${styles.checkbox} ${checkedIds.length > 0 ? styles.checkboxVisible : ''}`}
            checked={isChecked}
            aria-label={`Select ${payee.name}`}
            onClick={(event) => event.stopPropagation()}
            onChange={() => toggle(id)}
          />
        )}
        <PayeeAvatar name={payee.name} transfer={isTransfer} />
        <div className={styles.payeeRowText}>
          <span className={styles.payeeRowName}>
            <Highlight text={payee.name} query={query} />
          </span>
          <span className={styles.payeeRowMeta}>
            {ruleHit ? (
              <>
                Rule <code><Highlight text={ruleHit} query={query} /></code>
              </>
            ) : count > 0 ? (
              `${count} ${count === 1 ? 'transaction' : 'transactions'}${lastUsed ? ` · ${lastUsed}` : ''}`
            ) : isTransfer ? (
              'Account transfer'
            ) : (
              'Not used yet'
            )}
          </span>
        </div>
        <div className={styles.payeeRowEnd}>
          {(stats?.totalOutflow ?? 0) > 0 && (
            <span className={styles.payeeRowAmount}>{formatBudgetAmount(stats!.totalOutflow)}</span>
          )}
          {(stats?.ruleCount ?? 0) > 0 && (
            <span className={styles.rulePill} title={`${stats!.ruleCount} rules`}>
              <ListChecks size={12} weight="bold" />
              {stats!.ruleCount}
            </span>
          )}
        </div>
        <CaretRight size={14} className={styles.payeeRowCaret} />
      </div>
    );
  };

  return (
    <div className={styles.listPanel}>
      <div className={styles.listToolbar}>
        <div className={styles.chips} role="tablist" aria-label="Filter payees">
          {FILTERS.filter((item) => item.id === 'all' || item.id === filter || counts[item.id] > 0).map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={filter === item.id}
              className={`${styles.chip} ${filter === item.id ? styles.chipActive : ''} ${item.id === 'duplicates' ? styles.chipAccent : ''}`}
              onClick={() => onFilterChange(item.id)}>
              {item.id === 'duplicates' && <Sparkle size={12} weight="fill" />}
              {item.label}
              <span className={styles.chipCount}>{counts[item.id]}</span>
            </button>
          ))}
        </div>
        <div className={styles.listHeaderRow}>
          {filter !== 'transfers' && checkable.length > 0 ? (
            <label className={styles.selectAll}>
              <input
                type="checkbox"
                className={`${styles.checkbox} ${styles.checkboxVisible}`}
                checked={allChecked}
                onChange={() =>
                  onCheckedChange(
                    allChecked
                      ? checkedIds.filter((id) => !checkable.includes(id))
                      : [...new Set([...checkedIds, ...checkable])],
                  )
                }
              />
              <span>{flatRows.length} shown</span>
            </label>
          ) : (
            <span className={styles.selectAll}>{flatRows.length} shown</span>
          )}
          {filter !== 'duplicates' && (
            <select
              className={styles.sortSelect}
              value={sort}
              aria-label="Sort payees"
              onChange={(event) => onSortChange(event.target.value as PayeeSort)}>
              <option value="name">Name</option>
              <option value="used">Most used</option>
              <option value="recent">Recently used</option>
              <option value="spend">Most spent</option>
            </select>
          )}
        </div>
      </div>

      <div className={styles.payeeList} ref={listRef} role="listbox" tabIndex={0} onKeyDown={handleKeyDown}>
        {loading && payees.length === 0 &&
          Array.from({ length: 8 }, (_, index) => <div key={index} className={styles.rowSkeleton} />)}

        {filter === 'duplicates'
          ? visibleGroups.map((group) => (
              <div key={group.key} className={styles.dupGroup}>
                <div className={styles.dupGroupHeader}>
                  <span>
                    {group.payees.length} payees look like <strong>{group.payees[0].name}</strong>
                  </span>
                  <button
                    type="button"
                    className={styles.dupMergeButton}
                    onClick={() => onMerge(group.payees.map((payee) => payee.id!), group.suggestedTargetId)}>
                    <GitMerge size={14} /> Review merge
                  </button>
                </div>
                {group.rows.map(renderRow)}
              </div>
            ))
          : rows.map(renderRow)}

        {!loading && flatRows.length === 0 && (
          <div className={styles.listEmpty}>
            {query
              ? `Nothing matches “${search.trim()}”.`
              : filter === 'duplicates'
                ? 'No likely duplicates. Nice and tidy.'
                : 'No payees here.'}
          </div>
        )}
      </div>

      {checkedIds.length > 0 && (
        <div className={styles.bulkBar} role="toolbar" aria-label="Selected payees">
          <span>
            <strong>{checkedIds.length}</strong> selected
          </span>
          <div className={styles.bulkActions}>
            <button
              type="button"
              className={styles.bulkButton}
              disabled={checkedIds.length < 2}
              title={checkedIds.length < 2 ? 'Select at least two payees to merge' : undefined}
              onClick={() => onMerge(checkedIds)}>
              <GitMerge size={15} /> Merge
            </button>
            <button type="button" className={`${styles.bulkButton} ${styles.bulkDanger}`} onClick={() => onDelete(checkedIds)}>
              <Trash size={15} /> Delete
            </button>
            <button type="button" className={styles.bulkIcon} aria-label="Clear selection" onClick={() => onCheckedChange([])}>
              <X size={15} />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
