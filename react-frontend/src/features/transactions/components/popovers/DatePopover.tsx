import { useRef, useState } from 'react';
import { Calendar } from '@heroui/calendar';
import { parseDate, type DateValue } from '@internationalized/date';
import { Popover } from '@/components/common/Popover/Popover';
import styles from './Popover.module.css';

interface Props {
  value: string;
  onClick: (id: string, name: string) => void;
}

export function DateDropdown({ value, onClick }: Props) {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLInputElement | null>(null);

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

  return (
    <div className={styles.popoverContainer}>
      <input
        ref={triggerRef}
        onFocus={() => setIsOpen(true)}
        onBlur={(e) => {
          const popoverContent = document.getElementById('popover-content');
          if (
            popoverContent &&
            popoverContent.contains(e.relatedTarget as Node)
          ) {
            return;
          }
          setIsOpen(false);
        }}
        className={`${styles.input} ${styles.trigger}`}
        value={value}
        readOnly
        placeholder="Select Date"
        aria-haspopup="true"
        aria-expanded={isOpen}
      />
      <Popover id="popover-content" isOpen={isOpen} triggerRef={triggerRef}>
        <div
          onMouseDown={(e) => e.preventDefault()}
        >
          <Calendar
            aria-label="Date Picker"
            value={dateValue}
            onChange={handleDateChange}
          />
        </div>
      </Popover>
    </div>
  );
}
