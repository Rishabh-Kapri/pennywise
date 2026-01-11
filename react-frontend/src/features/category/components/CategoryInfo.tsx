import { Pencil } from 'lucide-react';
import type { Category } from '../types/category.types';
import styles from './CategoryInfo.module.css';
import type React from 'react';
import { useEffect, useRef, useState } from 'react';
import { useDebounce } from '@/hooks/useDebounce';
import { useAppSelector } from '@/app/hooks';
import { selectSelectedMonth } from '@/features/budget';
import { getCurrencyLocaleString, getLocaleDate, getPreviousMonthKey } from '@/utils/date.utils';

interface CategoryInfoProps {
  category: Category | null;
}

interface ActivityInfoItemProps {
  title: string;
  amount: string;
}

function ActivityInfoItem({ title, amount }: ActivityInfoItemProps) {
  return (
    <div className={styles.infoItem}>
      <div className={styles.activityItemTitle}>{title}</div>
      <div className={styles.activityItemAmount}>{amount}</div>
    </div>
  );
}

export function CategoryInfo({ category }: CategoryInfoProps) {
  const [note, setNote] = useState<string>('');
  const month = useAppSelector(selectSelectedMonth);
  const localeMonth = getLocaleDate(month, { month: 'long' });
  const previousMonth = getPreviousMonthKey(month);
  console.log(month, previousMonth, localeMonth);

  const debouncedNote = useDebounce(note);

  // useRef for AbortController for cancelling previous request.
  // useRef doesn't re-render even if the value changes.
  const abortController = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!category) {
      return;
    }

    setNote(category.note || '');
  }, [category]);

  useEffect(() => {
    if (!category) {
      return;
    }
    if (debouncedNote === (category?.note ?? '')) {
      return;
    }
    if (abortController.current) {
      // Abort the previous request.
      abortController.current.abort();
    }
    const controller = new AbortController();
    abortController.current = controller;

    console.log('Updating note...', debouncedNote);
    // disaptch note update
    // dispatch();
  }, [debouncedNote, category]);

  const handleNoteChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    if (!category) {
      return;
    }
    setNote(event.target.value);
  };

  return (
    <div className={styles.wrapper}>
      <div className={styles.spacer}></div>
      <div className={styles.infoContainer}>
        {!category && <div>Please select a category</div>}
        {category && (
          <div className={styles.info}>
            <div className={styles.title}>
              <div>{category.name}</div>
              <Pencil className={styles.icon} />
            </div>
            <div className={styles.goalCard}></div>
            <div className={styles.budgetInfo}>
              <div className={styles.header}>
                <div>Available in {localeMonth}</div>
                <div>
                  {getCurrencyLocaleString(category.balance?.[month] ?? 0)}
                </div>
              </div>
              <hr className={styles.divider} />
              <ActivityInfoItem
                title="Left Over from Last Month"
                amount={getCurrencyLocaleString(category.balance?.[previousMonth] ?? 0)}
              />
              <ActivityInfoItem
                title={`Assigned in ${localeMonth}`}
                amount={getCurrencyLocaleString(
                  category.budgeted?.[month] ?? 0,
                )}
              />
              <ActivityInfoItem
                title="Activity"
                amount={getCurrencyLocaleString(
                  category.activity?.[month] ?? 0,
                )}
              />
            </div>
            <div className={styles.note}>
              <div className={styles.header}>Note</div>
              <textarea
                placeholder="Add a note"
                rows={7}
                value={note ?? ''}
                onChange={handleNoteChange}></textarea>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
