import { TrashIcon } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { selectAllTags } from '@/features/tags/store/tagSlice';
import { AccountDropdown } from '@/features/transactions/components/popovers/AccountPopover';
import { CategoryDropdown } from '@/features/transactions/components/popovers/CategoryPopover';
import { TagDropdown } from '@/features/transactions/components/popovers/TagPopover';
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
  const allTags = useAppSelector(selectAllTags);

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
        <TagDropdown
          selectedTagIds={filters.tagIds}
          allowCreate={false}
          placeholder={filters.tagNames.length > 0 ? filters.tagNames.join(', ') : 'Search tags'}
          onChange={(tagIds) =>
            update({
              tagIds,
              tagNames: allTags.filter((tag) => tagIds.includes(tag.id)).map((tag) => tag.name),
            })
          }
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
