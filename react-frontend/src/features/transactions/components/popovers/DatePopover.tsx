import { useState } from 'react';
import { Calendar } from '@heroui/calendar';
import { parseDate, type DateValue } from '@internationalized/date';
import { Popover, PopoverTrigger, PopoverContent } from '@heroui/popover';
import styles from './Popover.module.css';

interface Props {
  value: string;
  onClick: (id: string, name: string) => void;
}

export function DateDropdown({ value, onClick }: Props) {
  const [isOpen, setIsOpen] = useState(false);

  const handleDateChange = (newDate: DateValue) => {
    const dateString = newDate.toString();
    onClick(dateString, dateString); // For dates, id and name are the same
    setIsOpen(false);
  };

  let dateValue;
  try {
    dateValue = value ? parseDate(value) : undefined;
  } catch (e) {
    dateValue = undefined;
  }

  // Format the date for display
  const displayValue = value
    ? new Date(value).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
      })
    : '';

  return (
    <div className={styles.popoverContainer}>
      <Popover
        isOpen={isOpen}
        onOpenChange={setIsOpen}
        placement="bottom-start"
        showArrow
      >
        <PopoverTrigger>
          <input
            className={`${styles.input} ${styles.trigger}`}
            value={displayValue}
            readOnly
            placeholder="Select Date"
            aria-haspopup="true"
            aria-expanded={isOpen}
          />
        </PopoverTrigger>
        <PopoverContent className={styles.datePopoverContent}>
          <Calendar
            aria-label="Date Picker"
            value={dateValue}
            onChange={handleDateChange}
            showMonthAndYearPickers
          />
        </PopoverContent>
      </Popover>
    </div>
  );
}
