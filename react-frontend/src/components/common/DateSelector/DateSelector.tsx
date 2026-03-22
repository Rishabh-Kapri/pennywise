import { useState } from 'react';
import {
  Calendar as CalendarIcon,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  LoaderCircle,
} from 'lucide-react';
import { Popover, PopoverTrigger, PopoverContent } from '@heroui/popover';
import { Calendar } from '@heroui/calendar';
import { CalendarDate, type DateValue } from '@internationalized/date';
import styles from './DateSelector.module.css';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { selectMonthInHumanFormat, setSelectedMonth } from '@/features/budget';
import { getMonthKey } from '@/utils/date.utils';

interface DateSelectorProps {
  /** When true, only shows month and year pickers without the day calendar grid */
  monthYearOnly?: boolean;
}

const MONTHS = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December'
];

export default function DateSelector({ monthYearOnly = false }: DateSelectorProps) {
  const { loading } = useAppSelector((state) => state.budgets);
  const dispatch = useAppDispatch();
  const selectedMonth = useAppSelector(selectMonthInHumanFormat);
  const monthKey = useAppSelector((state) => state.budgets.selectedMonth);
  const [isOpen, setIsOpen] = useState(false);

  // Parse current year and month from monthKey
  const [currentYear, currentMonthNum] = monthKey.split('-').map(Number);

  // Convert monthKey (yyyy-mm) to CalendarDate
  const getCalendarDate = (): DateValue | undefined => {
    if (!monthKey) return undefined;
    try {
      const [year, month] = monthKey.split('-');
      // Calendar expects first day of the month
      return new CalendarDate(parseInt(year, 10), parseInt(month, 10), 1);
    } catch (e) {
      return undefined;
    }
  };

  // Get min and max dates for reasonable year range (current year ± 5 years)
  const getMinMaxDates = () => {
    return {
      minValue: new CalendarDate(currentYear - 5, 1, 1),
      maxValue: new CalendarDate(currentYear + 5, 12, 31),
    };
  };

  const handleMonthChange = (addMonth: number) => {
    const currentDate = new Date(currentYear, currentMonthNum - 1);
    currentDate.setMonth(currentDate.getMonth() + addMonth);
    const newYear = currentDate.getFullYear();
    const newMonth = currentDate.getMonth();

    dispatch(setSelectedMonth(getMonthKey(newYear, newMonth)));
  };

  const handleDateChange = (date: DateValue) => {
    // Extract year and month from the selected date
    const newMonthKey = getMonthKey(date.year, date.month - 1);
    dispatch(setSelectedMonth(newMonthKey));
    setIsOpen(false);
  };

  const handleMonthSelect = (monthIndex: number) => {
    dispatch(setSelectedMonth(getMonthKey(currentYear, monthIndex)));
    setIsOpen(false);
  };

  const handleYearChange = (yearDelta: number) => {
    const newYear = currentYear + yearDelta;
    dispatch(setSelectedMonth(getMonthKey(newYear, currentMonthNum - 1)));
  };

  const { minValue, maxValue } = getMinMaxDates();

  return (
    <div className={styles.container}>
      <ChevronLeft
        size="2rem"
        className={styles.icon}
        onClick={() => handleMonthChange(-1)}
      />
      <Popover
        isOpen={isOpen}
        onOpenChange={setIsOpen}
        placement="bottom"
        showArrow
      >
        <PopoverTrigger>
          <div className={styles.dateContainer}>
            <CalendarIcon className={styles.icon} size="1.25rem" />
            {loading === LoadingState.PENDING && (
              <LoaderCircle
                size="1.25rem"
                className={`${styles.icon} ${styles.spinner}`}
              />
            )}
            {(loading === LoadingState.SUCCESS ||
              loading === LoadingState.ERROR) && <div>{selectedMonth}</div>}
            <ChevronDown
              className={`${styles.icon} ${styles.chevron} ${isOpen ? styles.chevronOpen : ''}`}
              size="1.25rem"
            />
          </div>
        </PopoverTrigger>
        <PopoverContent className={styles.popoverContent}>
          {monthYearOnly ? (
            <div className={styles.monthYearPicker}>
              {/* Year selector */}
              <div className={styles.yearSelector}>
                <ChevronLeft
                  size="1.25rem"
                  className={styles.yearNavIcon}
                  onClick={() => handleYearChange(-1)}
                />
                <span className={styles.yearText}>{currentYear}</span>
                <ChevronRight
                  size="1.25rem"
                  className={styles.yearNavIcon}
                  onClick={() => handleYearChange(1)}
                />
              </div>
              {/* Month grid */}
              <div className={styles.monthGrid}>
                {MONTHS.map((month, index) => (
                  <button
                    key={month}
                    className={`${styles.monthButton} ${index + 1 === currentMonthNum ? styles.monthButtonActive : ''}`}
                    onClick={() => handleMonthSelect(index)}
                  >
                    {month.slice(0, 3)}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <div className={styles.calendarWrapper}>
              <Calendar
                aria-label="Select month and year"
                value={getCalendarDate()}
                onChange={handleDateChange}
                showMonthAndYearPickers
                minValue={minValue}
                maxValue={maxValue}
              />
            </div>
          )}
        </PopoverContent>
      </Popover>
      <ChevronRight
        size="2rem"
        className={styles.icon}
        onClick={() => handleMonthChange(1)}
      />
    </div>
  );
}
