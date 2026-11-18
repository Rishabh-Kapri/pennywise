import { XIcon } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { selectAllTags } from '@/features/tags/store/tagSlice';
import { tagColor, tagLabel } from '@/features/tags/utils';
import { getLocaleDate } from '@/utils/date.utils';
import { EMPTY_FILTERS, hasActiveFilters, type TransactionFilters } from '../../types/filter.types';
import styles from './ActiveFilterChips.module.css';

interface Chip {
  key: string;
  kind: string;
  label: string;
  color?: string;
  remove: () => void;
}

const formatDate = (iso: string) => getLocaleDate(iso, { day: 'numeric', month: 'short', year: 'numeric' }, ['en-GB']);

/**
 * One removable chip per applied value, so a filter can be taken off without
 * hunting for it in its dropdown, and each date can be cleared on its own.
 */
export function ActiveFilterChips({
  filters,
  onChange,
  hideAccounts = false,
}: {
  filters: TransactionFilters;
  onChange: (filters: TransactionFilters) => void;
  hideAccounts?: boolean;
}) {
  const allPayees = useAppSelector((state) => state.payees.allPayees);
  const allAccounts = useAppSelector((state) => state.accounts.allAccounts);
  const allCategories = useAppSelector((state) => state.categories.allCategories);
  const allTags = useAppSelector(selectAllTags);

  if (!hasActiveFilters(filters)) return null;

  const set = (patch: Partial<TransactionFilters>) => onChange({ ...filters, ...patch });
  // Names are kept beside ids for the dropdown fields; rebuild them from the store.
  const namesOf = <T extends { id?: string; name: string }>(list: T[], ids: string[]) =>
    list.filter((item) => item.id && ids.includes(item.id)).map((item) => item.name);

  const chips: Chip[] = [];

  if (filters.dateFrom) {
    chips.push({ key: 'from', kind: 'From', label: formatDate(filters.dateFrom), remove: () => set({ dateFrom: '' }) });
  }
  if (filters.dateTo) {
    chips.push({ key: 'to', kind: 'To', label: formatDate(filters.dateTo), remove: () => set({ dateTo: '' }) });
  }
  if (filters.note.trim()) {
    chips.push({ key: 'note', kind: 'Note', label: `“${filters.note.trim()}”`, remove: () => set({ note: '' }) });
  }
  if (!hideAccounts) {
    for (const id of filters.accountIds) {
      const next = filters.accountIds.filter((x) => x !== id);
      chips.push({
        key: `account-${id}`,
        kind: 'Account',
        label: allAccounts.find((a) => a.id === id)?.name ?? 'Unknown account',
        remove: () => set({ accountIds: next, accountNames: namesOf(allAccounts, next) }),
      });
    }
  }
  for (const id of filters.payeeIds) {
    const next = filters.payeeIds.filter((x) => x !== id);
    chips.push({
      key: `payee-${id}`,
      kind: 'Payee',
      label: allPayees.find((p) => p.id === id)?.name ?? 'Unknown payee',
      remove: () => set({ payeeIds: next, payeeNames: namesOf(allPayees, next) }),
    });
  }
  for (const id of filters.categoryIds) {
    const next = filters.categoryIds.filter((x) => x !== id);
    chips.push({
      key: `category-${id}`,
      kind: 'Category',
      label: allCategories.find((c) => c.id === id)?.name ?? 'Unknown category',
      remove: () => set({ categoryIds: next, categoryNames: namesOf(allCategories, next) }),
    });
  }
  for (const id of filters.tagIds) {
    const next = filters.tagIds.filter((x) => x !== id);
    const tag = allTags.find((t) => t.id === id);
    chips.push({
      key: `tag-${id}`,
      kind: 'Tag',
      label: tag ? tagLabel(tag.name) : 'Unknown tag',
      color: tag ? tagColor(tag) : undefined,
      remove: () =>
        set({
          tagIds: next,
          tagNames: allTags.filter((t) => next.includes(t.id)).map((t) => tagLabel(t.name)),
        }),
    });
  }
  if (filters.pendingOnly) {
    chips.push({ key: 'pending', kind: 'Status', label: 'Pending approval', remove: () => set({ pendingOnly: false }) });
  }

  if (chips.length === 0) return null;

  return (
    <div className={styles.row} aria-label="Applied filters">
      {chips.map((chip) => (
        <span key={chip.key} className={styles.chip}>
          <span className={styles.kind}>{chip.kind}</span>
          {chip.color && <span className={styles.dot} style={{ backgroundColor: chip.color }} aria-hidden="true" />}
          <span className={styles.label} title={chip.label}>
            {chip.label}
          </span>
          <button
            type="button"
            className={styles.remove}
            onClick={chip.remove}
            aria-label={`Remove ${chip.kind.toLowerCase()} filter ${chip.label}`}>
            <XIcon size={12} weight="bold" />
          </button>
        </span>
      ))}
      {chips.length > 1 && (
        <button
          type="button"
          className={styles.clearAll}
          onClick={() => onChange({ ...EMPTY_FILTERS, logicMode: filters.logicMode })}>
          Clear all
        </button>
      )}
    </div>
  );
}
