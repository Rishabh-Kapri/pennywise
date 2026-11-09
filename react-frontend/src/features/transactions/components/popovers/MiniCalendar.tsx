import { type KeyboardEvent, useEffect, useMemo, useRef, useState } from 'react';
import { CaretLeftIcon, CaretRightIcon } from '@phosphor-icons/react';
import { getTodaysDate } from '@/utils/date.utils';
import styles from './MiniCalendar.module.css';

const WEEKDAYS = ['S', 'M', 'T', 'W', 'T', 'F', 'S'];
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

const pad = (n: number) => String(n).padStart(2, '0');
const toIso = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const fromIso = (iso: string) => {
  const [y, m, d] = iso.split('-').map(Number);
  return y && m && d ? new Date(y, m - 1, d) : null;
};
const addDays = (d: Date, days: number) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + days);
const addMonths = (d: Date, months: number) => {
  const target = new Date(d.getFullYear(), d.getMonth() + months, 1);
  const lastDay = new Date(target.getFullYear(), target.getMonth() + 1, 0).getDate();
  return new Date(target.getFullYear(), target.getMonth(), Math.min(d.getDate(), lastDay));
};

interface MiniCalendarProps {
  /** YYYY-MM-DD */
  value: string;
  onSelect: (iso: string) => void;
}

/**
 * Plain month grid for the date pickers. Replaces HeroUI's Calendar, which
 * took 100ms+ to mount and made the picker feel slow to open. Click the
 * title to jump months/years; arrows, PageUp/PageDown and Enter work too.
 */
export function MiniCalendar({ value, onSelect }: MiniCalendarProps) {
  const today = getTodaysDate();
  const selected = fromIso(value);
  const [focused, setFocused] = useState<Date>(() => selected ?? fromIso(today)!);
  const [mode, setMode] = useState<'days' | 'months'>('days');
  const gridRef = useRef<HTMLDivElement | null>(null);

  // 6 weeks from the Sunday on/before the 1st
  const days = useMemo(() => {
    const first = new Date(focused.getFullYear(), focused.getMonth(), 1);
    const start = addDays(first, -first.getDay());
    return Array.from({ length: 42 }, (_, i) => addDays(start, i));
  }, [focused]);

  const focusedIso = toIso(focused);

  useEffect(() => {
    if (mode !== 'days') return;
    gridRef.current?.querySelector<HTMLButtonElement>(`[data-date="${focusedIso}"]`)?.focus({ preventScroll: true });
  }, [focusedIso, mode]);

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const moves: Record<string, () => Date> = {
      ArrowLeft: () => addDays(focused, -1),
      ArrowRight: () => addDays(focused, 1),
      ArrowUp: () => addDays(focused, -7),
      ArrowDown: () => addDays(focused, 7),
      PageUp: () => addMonths(focused, -1),
      PageDown: () => addMonths(focused, 1),
      Home: () => addDays(focused, -focused.getDay()),
      End: () => addDays(focused, 6 - focused.getDay()),
    };
    const move = moves[event.key];
    if (!move) return;
    event.preventDefault();
    setFocused(move());
  };

  const month = focused.getMonth();
  const year = focused.getFullYear();

  return (
    <div className={styles.calendar}>
      <div className={styles.header}>
        <button
          type="button"
          className={styles.navButton}
          aria-label={mode === 'days' ? 'Previous month' : 'Previous year'}
          onClick={() => setFocused(addMonths(focused, mode === 'days' ? -1 : -12))}>
          <CaretLeftIcon size={15} weight="bold" />
        </button>
        <button
          type="button"
          className={styles.title}
          aria-label={mode === 'days' ? 'Choose month and year' : 'Back to days'}
          onClick={() => setMode(mode === 'days' ? 'months' : 'days')}>
          {mode === 'days' ? `${MONTHS[month]} ${year}` : year}
        </button>
        <button
          type="button"
          className={styles.navButton}
          aria-label={mode === 'days' ? 'Next month' : 'Next year'}
          onClick={() => setFocused(addMonths(focused, mode === 'days' ? 1 : 12))}>
          <CaretRightIcon size={15} weight="bold" />
        </button>
      </div>

      {mode === 'months' ? (
        <div className={styles.monthGrid}>
          {MONTHS.map((name, index) => (
            <button
              key={name}
              type="button"
              className={`${styles.month} ${index === month ? styles.isSelected : ''}`}
              onClick={() => {
                setFocused(addMonths(focused, index - month));
                setMode('days');
              }}>
              {name}
            </button>
          ))}
        </div>
      ) : (
        <div ref={gridRef} className={styles.grid} role="grid" aria-label={`${MONTHS[month]} ${year}`} onKeyDown={handleKeyDown}>
          {WEEKDAYS.map((day, index) => (
            <span key={index} className={styles.weekday} aria-hidden="true">
              {day}
            </span>
          ))}
          {days.map((day) => {
            const iso = toIso(day);
            const isSelected = iso === value;
            return (
              <button
                key={iso}
                type="button"
                data-date={iso}
                tabIndex={iso === focusedIso ? 0 : -1}
                aria-pressed={isSelected}
                aria-label={day.toDateString()}
                className={`${styles.day} ${day.getMonth() !== month ? styles.outside : ''} ${
                  iso === today ? styles.today : ''
                } ${isSelected ? styles.isSelected : ''}`}
                onClick={() => onSelect(iso)}>
                {day.getDate()}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
