import { useCallback } from 'react';
import { XIcon } from '@phosphor-icons/react';
import { AccountDropdown } from '@/features/accounts/components/AccountDropdown';
import { PayeeDropdown } from '@/features/payees/components/PayeeDropdown';
import { CategoryDropdown } from '@/features/category/components/CategoryDropdown';
import { DateDropdown } from '@/components/common/pickers/DateDropdown';
import { TagFilterDropdown } from '@/features/tags/components/TagFilterDropdown';
import type { TransactionFacets } from '../../hooks/useTransactionFacets';
import {
  EMPTY_FILTERS,
  hasActiveFilters,
  type FilterLogicMode,
  type TransactionFilters,
} from '../../types/filter.types';
import styles from './TransactionFilterPanel.module.css';

interface TransactionFilterPanelProps {
  filters: TransactionFilters;
  onChange: (filters: TransactionFilters) => void;
  hideAccountFilter?: boolean;
  /** What the current results contain, listed first in the payee, account, category and tag menus. */
  facets?: TransactionFacets | null;
}

export function TransactionFilterPanel({
  filters,
  onChange,
  hideAccountFilter = false,
  facets,
}: TransactionFilterPanelProps) {
  const isActive = hasActiveFilters(filters);

  const update = useCallback(
    (patch: Partial<TransactionFilters>) => onChange({ ...filters, ...patch }),
    [filters, onChange],
  );

  const clearAll = useCallback(() => onChange(EMPTY_FILTERS), [onChange]);

  const setLogicMode = useCallback((mode: FilterLogicMode) => update({ logicMode: mode }), [update]);

  return (
    <div className={styles.filterRow}>
      {/* Note */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Note</label>
        <input
          type="text"
          className={styles.filterInput}
          placeholder="Contains…"
          aria-label="Note contains"
          value={filters.note}
          onChange={(e) => update({ note: e.target.value })}
        />
      </div>

      {/* Date from */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>From</label>
        <DateDropdown value={filters.dateFrom} onClick={(date) => update({ dateFrom: date })} variant="form" placeholder="Any date" />
      </div>

      {/* Date to */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>To</label>
        <DateDropdown value={filters.dateTo} onClick={(date) => update({ dateTo: date })} variant="form" placeholder="Any date" />
      </div>

      {/* Account */}
      {!hideAccountFilter && <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Account</label>
        <AccountDropdown
          multiple
          selectedIds={filters.accountIds}
          value={filters.accountNames.join(', ')}
          onChangeMultiple={(ids, names) => update({ accountIds: ids, accountNames: names })}
          onClick={() => { }}
          variant="form"
          placeholder="Any account"
          counts={facets?.accounts}
        />
      </div>}

      {/* Payee */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Payee</label>
        <PayeeDropdown
          multiple
          selectedIds={filters.payeeIds}
          value={filters.payeeNames.join(', ')}
          onChangeMultiple={(ids, names) => update({ payeeIds: ids, payeeNames: names })}
          onClick={() => { }}
          variant="form"
          placeholder="Any payee"
          counts={facets?.payees}
        />
      </div>

      {/* Category */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Category</label>
        <CategoryDropdown
          multiple
          selectedIds={filters.categoryIds}
          value={filters.categoryNames.join(', ')}
          onChangeMultiple={(ids, names) => update({ categoryIds: ids, categoryNames: names })}
          onClick={() => { }}
          variant="form"
          placeholder="Any category"
          counts={facets?.categories}
        />
      </div>

      {/* Tags */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Tags</label>
        <TagFilterDropdown
          selectedIds={filters.tagIds}
          placeholder="Any tag"
          counts={facets?.tags}
          onChange={(tagIds, tagNames) => update({ tagIds, tagNames })}
        />
      </div>

      {/* Match mode + clear */}
      <div className={styles.filterGroup}>
        <span className={styles.filterLabel}>Match</span>
        <div className={styles.filterActions}>
          <div className={styles.logicToggle} role="radiogroup" aria-label="Match filters">
            {(['AND', 'OR'] as const).map((mode) => (
              <button
                key={mode}
                type="button"
                role="radio"
                aria-checked={filters.logicMode === mode}
                className={`${styles.logicBtn} ${filters.logicMode === mode ? styles.logicBtnActive : ''}`}
                title={mode === 'AND' ? 'Transactions matching every filter' : 'Transactions matching any filter'}
                onClick={() => setLogicMode(mode)}>
                {mode === 'AND' ? 'All' : 'Any'}
              </button>
            ))}
          </div>
          {isActive && (
            <button type="button" className={styles.clearBtn} onClick={clearAll}>
              <XIcon size={13} weight="bold" />
              Clear
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
