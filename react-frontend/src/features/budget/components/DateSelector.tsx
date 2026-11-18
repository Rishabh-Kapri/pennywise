import { useState } from 'react';
import { Calendar as CalendarIcon, CaretDown as ChevronDown, CaretLeft as ChevronLeft, CaretRight as ChevronRight, CircleNotch as LoaderCircle } from '@phosphor-icons/react';
import { Popover } from '@heroui/react';
import { Button } from 'react-aria-components';
import styles from './DateSelector.module.css';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { selectMonthInHumanFormat, setSelectedMonth } from '@/features/budget';
import { getMonthKey } from '@/utils/date.utils';

const MONTHS = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December'
];

/** Month switcher: arrows either side, and a popover with a year stepper and month grid. */
export default function DateSelector() {
  const { loading } = useAppSelector((state) => state.budgets);
  const dispatch = useAppDispatch();
  const selectedMonth = useAppSelector(selectMonthInHumanFormat);
  const monthKey = useAppSelector((state) => state.budgets.selectedMonth);
  const [isOpen, setIsOpen] = useState(false);

  // Parse current year and month from monthKey
  const [currentYear, currentMonthNum] = monthKey.split('-').map(Number);

  const handleMonthChange = (addMonth: number) => {
    const currentDate = new Date(currentYear, currentMonthNum - 1);
    currentDate.setMonth(currentDate.getMonth() + addMonth);
    const newYear = currentDate.getFullYear();
    const newMonth = currentDate.getMonth();

    dispatch(setSelectedMonth(getMonthKey(newYear, newMonth)));
  };

  const handleMonthSelect = (monthIndex: number) => {
    dispatch(setSelectedMonth(getMonthKey(currentYear, monthIndex)));
    setIsOpen(false);
  };

  const handleYearChange = (yearDelta: number) => {
    const newYear = currentYear + yearDelta;
    dispatch(setSelectedMonth(getMonthKey(newYear, currentMonthNum - 1)));
  };

  return (
    <div className={styles.container}>
      <button
        type="button"
        className={styles.navButton}
        aria-label="Previous month"
        onClick={() => handleMonthChange(-1)}>
        <ChevronLeft size="1rem" />
      </button>
      <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
        <Button className={styles.dateContainer}>
          {loading === LoadingState.PENDING ? (
            <LoaderCircle size="1rem" className={`${styles.icon} ${styles.spinner}`} />
          ) : (
            <CalendarIcon className={styles.icon} size="1rem" />
          )}
          <span className={styles.label}>{selectedMonth}</span>
          <ChevronDown
            className={`${styles.icon} ${styles.chevron} ${isOpen ? styles.chevronOpen : ''}`}
            size="0.85rem"
          />
        </Button>
        <Popover.Content placement="bottom" className={styles.popoverContent}>
          <Popover.Arrow />
          <Popover.Dialog aria-label="Choose month">
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
          </Popover.Dialog>
        </Popover.Content>
      </Popover>
      <button
        type="button"
        className={styles.navButton}
        aria-label="Next month"
        onClick={() => handleMonthChange(1)}>
        <ChevronRight size="1rem" />
      </button>
    </div>
  );
}
