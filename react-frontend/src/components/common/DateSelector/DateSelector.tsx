import {
  Calendar,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  LoaderCircle,
} from 'lucide-react';
import styles from './DateSelector.module.css';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { selectMonthInHumanFormat, setSelectedMonth } from '@/features/budget';
import { getMonthKey } from '@/utils/date.utils';

export default function DateSelector() {
  const { loading } = useAppSelector((state) => state.budgets);
  const dispatch = useAppDispatch();
  const selectedMonth = useAppSelector(selectMonthInHumanFormat);
  const monthKey = useAppSelector((state) => state.budgets.selectedMonth);

  const handleMonthChange = (addMonth: number) => {
    const [year, month] = monthKey.split('-');
    const currentDate = new Date(parseInt(year, 10), parseInt(month, 10) - 1);
    currentDate.setMonth(currentDate.getMonth() + addMonth);
    const newYear = currentDate.getFullYear();
    const newMonth = currentDate.getMonth();

    // the dateChangeMiddleware will handle fetching the budget data for the dispatched month
    dispatch(setSelectedMonth(getMonthKey(newYear, newMonth)));
  };

  return (
    <div className={styles.container}>
      <ChevronLeft
        size="2rem"
        className={styles.icon}
        onClick={() => handleMonthChange(-1)}
      />
      <div className={styles.dateContainer}>
        <Calendar className={styles.icon} size="1.25rem" />
        {loading === LoadingState.PENDING && (
          <LoaderCircle
            size="1.25rem"
            className={`${styles.icon} ${styles.spinner}`}
          />
        )}
        {(loading === LoadingState.SUCCESS ||
          loading === LoadingState.ERROR) && <div>{selectedMonth}</div>}
        <ChevronDown className={styles.icon} size="1.25rem" />
      </div>
      <ChevronRight
        size="2rem"
        className={styles.icon}
        onClick={() => handleMonthChange(1)}
      />
    </div>
  );
}
