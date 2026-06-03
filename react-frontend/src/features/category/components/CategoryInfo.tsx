import {
  CalendarDotsIcon as CalendarDays,
  CurrencyCircleDollarIcon,
  NotePencilIcon,
  Pencil,
  TrendUpIcon,
  WalletIcon,
} from '@phosphor-icons/react';
import type { Category } from '../types/category.types';
import styles from './CategoryInfo.module.css';
import { Activity } from '@/features/budget/components/Activity';
import { ActivityPopover } from './ActivityModal';
import type React from 'react';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useDebounce } from '@/hooks/useDebounce';
import { useAppSelector } from '@/app/hooks';
import { selectSelectedMonth } from '@/features/budget';
import { getCurrencyLocaleString, getLocaleDate, getPreviousMonthKey } from '@/utils/date.utils';

interface CategoryInfoProps {
  category: Category | null;
}

function MetaItem({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <div className={styles.metaItem}>
      {icon}
      <strong className={styles.metaValue}>{children}</strong>
    </div>
  );
}

function getAmountClass(value: number) {
  if (value > 0) return styles.heroAmountPositive;
  if (value < 0) return styles.heroAmountNegative;
  return styles.heroAmountZero;
}

export function CategoryInfo({ category }: CategoryInfoProps) {
  const [note, setNote] = useState<string>('');
  const month = useAppSelector(selectSelectedMonth);
  const localeMonth = getLocaleDate(month, { month: 'long' });
  const previousMonth = getPreviousMonthKey(month);
  const [showActivityModal, setShowActivityModal] = useState(false);
  const activityTriggerRef = useRef<HTMLDivElement | null>(null);

  const [debouncedNote] = useDebounce(note);

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

  const available = category?.balance?.[month] ?? 0;
  const leftOver = category?.balance?.[previousMonth] ?? 0;
  const assigned = category?.budgeted?.[month] ?? 0;
  const activity = category?.activity?.[month] ?? 0;

  return (
    <div className={styles.wrapper}>
      <div className={styles.spacer}></div>
      <div className={styles.infoContainer}>
        {!category && (
          <div className={styles.defaultContent}>
            <Activity />
          </div>
        )}
        {category && (
          <div className={styles.panel}>
            {/* ── Header ── */}
            <div className={styles.panelHeader}>
              <span className={styles.panelTitle}>{category.name}</span>
              <button type="button" className={styles.editButton} aria-label="Edit category">
                <Pencil size={18} />
              </button>
            </div>

            {/* ── Body ── */}
            <div className={styles.panelBody}>
              {/* Hero: Available balance */}
              <section className={styles.heroSection}>
                <span className={styles.heroLabel}>Available in {localeMonth}</span>
                <div className={`${styles.heroAmount} ${getAmountClass(available)}`}>
                  {getCurrencyLocaleString(available)}
                </div>
              </section>

              {/* Meta grid: Budget breakdown */}
              <section className={styles.metaGrid}>
                <MetaItem icon={<CalendarDays color="var(--color-text)" size={18} />}>
                  {getCurrencyLocaleString(leftOver)}
                  <span style={{ fontWeight: 400, fontSize: '0.82rem', color: 'var(--color-text-secondary)', marginLeft: '0.4rem' }}>
                    left over
                  </span>
                </MetaItem>
                <MetaItem icon={<CurrencyCircleDollarIcon color="var(--color-text)" size={18} />}>
                  {getCurrencyLocaleString(assigned)}
                  <span style={{ fontWeight: 400, fontSize: '0.82rem', color: 'var(--color-text-secondary)', marginLeft: '0.4rem' }}>
                    assigned
                  </span>
                </MetaItem>
              </section>

              {/* Activity */}
              <section
                ref={activityTriggerRef}
                className={styles.activitySection}
                onClick={() => setShowActivityModal(true)}
              >
                <span className={styles.metaLabel}>
                  <TrendUpIcon size={18} />
                  <span>Activity</span>
                </span>
                <span className={styles.activityAmount}>
                  {getCurrencyLocaleString(activity)}
                </span>
              </section>

              {/* Goal placeholder */}
              <section className={styles.goalSection}>
                <span className={styles.metaLabel}>
                  <WalletIcon size={18} />
                  <span>Goal</span>
                </span>
                <div className={styles.goalCard}>No goal set</div>
              </section>

              {/* Notes */}
              <section className={styles.notesSection}>
                <span className={styles.metaLabel}>
                  <NotePencilIcon color="var(--color-text)" size={18} />
                  <span>Notes</span>
                </span>
                <textarea
                  className={styles.notesInput}
                  placeholder="Add a note about this category..."
                  rows={5}
                  value={note ?? ''}
                  onChange={handleNoteChange}
                />
              </section>
            </div>

            {/* Activity popover */}
            <ActivityPopover
              isOpen={showActivityModal}
              onClose={() => setShowActivityModal(false)}
              triggerRef={activityTriggerRef}
              categoryId={category.id ?? ''}
              categoryName={category.name}
              month={month}
              activityAmount={activity}
            />
          </div>
        )}
      </div>
    </div>
  );
}
