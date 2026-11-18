import { TrashIcon } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { AccountDropdown } from '@/features/accounts/components/AccountDropdown';
import { CategoryDropdown } from '@/features/category/components/CategoryDropdown';
import { TagFilterDropdown } from '@/features/tags/components/TagFilterDropdown';
import { selectReportFilters, setReportFilters } from '../store/reportSlice';
import {
  EMPTY_REPORT_FILTERS,
  hasActiveReportFilters,
  type ReportFilters,
} from '../types/report.types';
import styles from './ReportFilterPanel.module.css';

export default function ReportFilterPanel() {
  const dispatch = useAppDispatch();
  const filters = useAppSelector(selectReportFilters);

  const update = (patch: Partial<ReportFilters>) =>
    dispatch(setReportFilters({ ...filters, ...patch }));

  return (
    <div className={styles.filterRow}>
      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Accounts</label>
        <AccountDropdown
          multiple
          selectedIds={filters.accountIds}
          value={filters.accountNames.join(', ')}
          onChangeMultiple={(ids, names) => update({ accountIds: ids, accountNames: names })}
          onClick={() => {}}
          variant="form"
        />
      </div>

      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Categories</label>
        <CategoryDropdown
          multiple
          selectedIds={filters.categoryIds}
          value={filters.categoryNames.join(', ')}
          onChangeMultiple={(ids, names) => update({ categoryIds: ids, categoryNames: names })}
          onClick={() => {}}
          variant="form"
        />
      </div>

      <div className={styles.filterGroup}>
        <label className={styles.filterLabel}>Tags</label>
        <TagFilterDropdown
          selectedIds={filters.tagIds}
          placeholder="Any tag"
          onChange={(tagIds, tagNames) => update({ tagIds, tagNames })}
        />
      </div>

      {hasActiveReportFilters(filters) && (
        <button
          type="button"
          className={styles.clearBtn}
          onClick={() => dispatch(setReportFilters(EMPTY_REPORT_FILTERS))}
          title="Clear filters">
          <TrashIcon size={14} />
          Clear
        </button>
      )}
    </div>
  );
}
