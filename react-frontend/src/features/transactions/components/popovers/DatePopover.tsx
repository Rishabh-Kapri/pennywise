import { useEffect, useRef, useState } from 'react';
import { Calendar } from '@heroui/calendar';
import { parseDate, type DateValue } from '@internationalized/date';
import { Popover, PopoverTrigger, PopoverContent } from '@heroui/popover';
import styles from './Popover.module.css';
import { getLocaleDate } from '@/utils/date.utils';
import type { TransactionDropdownProps } from './types';

export function DateDropdown({ value, onClick, autoFocus, variant = 'inline' }: TransactionDropdownProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [pendingDate, setPendingDate] = useState<DateValue | undefined>();
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  const getDateValue = (date: string) => {
    try {
      return date ? parseDate(date) : undefined;
    } catch {
      return undefined;
    }
  };

  useEffect(() => {
    setPendingDate(getDateValue(value));
  }, [value]);

  useEffect(() => {
    if (autoFocus) {
      setIsOpen(true);
    }
  }, [autoFocus]);

  const handleOpenChange = (open: boolean) => {
    setIsOpen(open);
    if (!open) {
      triggerRef.current?.blur();
    }
  };

  const handleConfirm = () => {
    if (!pendingDate) return;
    const dateString = pendingDate.toString();
    onClick(dateString, dateString);
    handleOpenChange(false);
  };

  // Format the date for display
  const displayValue = value
    ? getLocaleDate(value, { month: 'short', day: 'numeric', year: 'numeric' }, ['en-GB'])
    : '';
  const triggerClassName = variant === 'form' ? styles.formTrigger : styles.dateTrigger;

  return (
    <div className={styles.popoverContainer}>
      <Popover isOpen={isOpen} onOpenChange={handleOpenChange} placement="bottom-start" showArrow>
        <PopoverTrigger>
          <button
            type="button"
            ref={triggerRef}
            className={triggerClassName}
            autoFocus={autoFocus}
            aria-haspopup="true"
            aria-expanded={isOpen}>
            {displayValue || 'Select Date'}
          </button>
        </PopoverTrigger>
        <PopoverContent className={styles.datePopoverContent}>
          <Calendar
            aria-label="Date Picker"
            value={pendingDate}
            onChange={setPendingDate}
            showMonthAndYearPickers
            classNames={{
              base: styles.calendarBase,
              content: styles.calendarContent,
              header: styles.calendarHeader,
              headerWrapper: styles.calendarHeaderWrapper,
              title: styles.calendarTitle,
              prevButton: styles.calendarNavButton,
              nextButton: styles.calendarNavButton,
              gridWrapper: styles.calendarGridWrapper,
              grid: styles.calendarGrid,
              gridHeader: styles.calendarGridHeader,
              gridHeaderCell: styles.calendarWeekday,
              gridBody: styles.calendarGridBody,
              cellButton: styles.calendarDayButton,
              pickerWrapper: styles.calendarPickerWrapper,
              pickerMonthList: styles.calendarPickerList,
              pickerYearList: styles.calendarPickerList,
              pickerHighlight: styles.calendarPickerHighlight,
              pickerItem: styles.calendarPickerItem,
            }}
          />
          <div className={styles.dateActions}>
            <button
              type="button"
              className={styles.dateOkButton}
              disabled={!pendingDate}
              onClick={handleConfirm}>
              Ok
            </button>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
