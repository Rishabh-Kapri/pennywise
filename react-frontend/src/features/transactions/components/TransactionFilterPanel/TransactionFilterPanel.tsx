import { useCallback } from 'react';
import { TrashIcon } from '@phosphor-icons/react';
import { AccountDropdown } from '../popovers/AccountPopover';
import { PayeeDropdown } from '../popovers/PayeePopover';
import { CategoryDropdown } from '../popovers/CategoryPopover';
import { DateDropdown } from '../popovers/DatePopover';
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
}

export function TransactionFilterPanel({ filters, onChange }: TransactionFilterPanelProps) {
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
          value={filters.note}
          onChange={(e) => update({ note: e.target.value })}
        />
      </div>

      {/* Date from */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>From</label>
        <DateDropdown value={filters.dateFrom} onClick={(date) => update({ dateFrom: date })} variant="form" />
      </div>

      {/* Date to */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>To</label>
        <DateDropdown value={filters.dateTo} onClick={(date) => update({ dateTo: date })} variant="form" />
      </div>

      {/* Account */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Account</label>
        <AccountDropdown
          multiple
          selectedIds={filters.accountIds}
          value={filters.accountNames.join(', ')}
          onChangeMultiple={(ids, names) => update({ accountIds: ids, accountNames: names })}
          onClick={() => { }}
          variant="form"
        />
      </div>

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
        />
      </div>

      {/* Logic toggle + clear */}
      <div className={styles.filterActions}>
        <div className={styles.logicToggle}>
          <button
            type="button"
            className={`${styles.logicBtn} ${filters.logicMode === 'AND' ? styles.logicBtnActive : ''}`}
            onClick={() => setLogicMode('AND')}>
            AND
          </button>
          <button
            type="button"
            className={`${styles.logicBtn} ${filters.logicMode === 'OR' ? styles.logicBtnActive : ''}`}
            onClick={() => setLogicMode('OR')}>
            OR
          </button>
        </div>
        {isActive && (
          <button type="button" className={styles.clearBtn} onClick={clearAll} title="Clear all filters">
            <TrashIcon size={14} />
          </button>
        )}
      </div>
    </div>
  );
}
