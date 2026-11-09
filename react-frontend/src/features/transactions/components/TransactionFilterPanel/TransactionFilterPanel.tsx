import { useCallback } from 'react';
import { XIcon } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { selectAllTags } from '@/features/tags/store/tagSlice';
import { AccountDropdown } from '../popovers/AccountPopover';
import { PayeeDropdown } from '../popovers/PayeePopover';
import { CategoryDropdown } from '../popovers/CategoryPopover';
import { DateDropdown } from '../popovers/DatePopover';
import { TagDropdown } from '../popovers/TagPopover';
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
}

export function TransactionFilterPanel({ filters, onChange, hideAccountFilter = false }: TransactionFilterPanelProps) {
  const isActive = hasActiveFilters(filters);
  const allTags = useAppSelector(selectAllTags);

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
        />
      </div>

      {/* Tags */}
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Tags</label>
        <TagDropdown
          selectedTagIds={filters.tagIds}
          allowCreate={false}
          variant="form"
          placeholder={filters.tagNames.length > 0 ? filters.tagNames.join(', ') : 'Any tag'}
          onChange={(tagIds) =>
            update({
              tagIds,
              tagNames: allTags.filter((tag) => tagIds.includes(tag.id)).map((tag) => tag.name),
            })
          }
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
