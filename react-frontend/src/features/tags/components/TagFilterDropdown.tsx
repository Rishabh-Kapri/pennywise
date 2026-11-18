import { useMemo } from 'react';
import { useAppSelector } from '@/app/hooks';
import { selectAllTags } from '@/features/tags/store/tagSlice';
import { tagColor, tagLabel } from '@/features/tags/utils';
import { MultiSelectMenu } from '@/components/common/pickers/MultiSelectMenu';
import styles from '@/components/common/pickers/MultiSelectMenu.module.css';

interface TagFilterDropdownProps {
  selectedIds: string[];
  onChange: (ids: string[], labels: string[]) => void;
  placeholder: string;
  counts?: Record<string, number>;
}

/** Tag picker for the filter trays. Picking only: tags are created on transactions. */
export function TagFilterDropdown({ selectedIds, onChange, placeholder, counts }: TagFilterDropdownProps) {
  const allTags = useAppSelector(selectAllTags);
  const items = useMemo(
    () =>
      [...allTags]
        .sort((a, b) => tagLabel(a.name).localeCompare(tagLabel(b.name)))
        .map((tag) => ({
          id: tag.id,
          name: tagLabel(tag.name),
          searchText: tag.name,
          leading: (
            <span className={styles.tagDot} style={{ backgroundColor: tagColor(tag) }} />
          ),
        })),
    [allTags],
  );

  return (
    <MultiSelectMenu
      id="tag-filter-menu"
      items={items}
      selectedIds={selectedIds}
      onChange={(ids) => onChange(ids, items.filter((item) => ids.includes(item.id)).map((item) => item.name))}
      placeholder={placeholder}
      noun="tags"
      counts={counts}
    />
  );
}
