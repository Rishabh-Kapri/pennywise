import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { addMonths, getCurrentMonthKey } from '@/utils/date.utils';
import { selectReportPreset, selectReportRange, setRange } from '../store/reportSlice';
import type { ReportPreset } from '../types/report.types';
import styles from './ReportRangePicker.module.css';

// generous floor for the "All" preset; the backend clamps to actual data
const ALL_TIME_START = '2020-01';

const PRESETS: { key: Exclude<ReportPreset, 'custom'>; label: string }[] = [
  { key: '3m', label: '3M' },
  { key: '6m', label: '6M' },
  { key: '12m', label: '12M' },
  { key: 'ytd', label: 'YTD' },
  { key: 'all', label: 'All' },
];

function presetRange(preset: Exclude<ReportPreset, 'custom'>) {
  const endMonth = getCurrentMonthKey();
  switch (preset) {
    case '3m':
      return { startMonth: addMonths(endMonth, -2), endMonth };
    case '6m':
      return { startMonth: addMonths(endMonth, -5), endMonth };
    case '12m':
      return { startMonth: addMonths(endMonth, -11), endMonth };
    case 'ytd':
      return { startMonth: `${endMonth.split('-')[0]}-01`, endMonth };
    case 'all':
      return { startMonth: ALL_TIME_START, endMonth };
  }
}

export default function ReportRangePicker() {
  const dispatch = useAppDispatch();
  const range = useAppSelector(selectReportRange);
  const preset = useAppSelector(selectReportPreset);

  const handleCustomChange = (field: 'startMonth' | 'endMonth', value: string) => {
    if (!value) return;
    const next = { ...range, [field]: value };
    if (next.startMonth > next.endMonth) return;
    dispatch(setRange({ range: next, preset: 'custom' }));
  };

  return (
    <div className={styles.container}>
      <div className={styles.presets}>
        {PRESETS.map(({ key, label }) => (
          <button
            key={key}
            className={preset === key ? styles.presetActive : styles.preset}
            onClick={() => dispatch(setRange({ range: presetRange(key), preset: key }))}>
            {label}
          </button>
        ))}
      </div>
      <div className={styles.custom}>
        <input
          type="month"
          className={styles.monthInput}
          value={range.startMonth}
          max={range.endMonth}
          onChange={(e) => handleCustomChange('startMonth', e.target.value)}
          aria-label="Start month"
        />
        <span className={styles.rangeSeparator}>–</span>
        <input
          type="month"
          className={styles.monthInput}
          value={range.endMonth}
          min={range.startMonth}
          onChange={(e) => handleCustomChange('endMonth', e.target.value)}
          aria-label="End month"
        />
      </div>
    </div>
  );
}
