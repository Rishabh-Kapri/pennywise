import { useEffect, useRef, useState } from 'react';
import { Popover } from '@/components/common/Popover/Popover';
import { MiniCalendar } from './MiniCalendar';
import styles from './Popover.module.css';
import { getLocaleDate } from '@/utils/date.utils';
import type { DropdownVariant, TransactionDropdownProps } from './types';

const triggerClass: Record<DropdownVariant, string> = {
  inline: styles.dateTrigger,
  form: styles.formTrigger,
  ghost: styles.ghostTrigger,
};

export function DateDropdown({ value, onClick, autoFocus, variant = 'inline', placeholder }: TransactionDropdownProps) {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (autoFocus) setIsOpen(true);
  }, [autoFocus]);

  const close = () => {
    setIsOpen(false);
    triggerRef.current?.focus();
  };

  const displayValue = value
    ? getLocaleDate(value, { month: 'short', day: 'numeric', year: 'numeric' }, ['en-GB'])
    : '';

  return (
    <div className={`${styles.popoverContainer} ${variant === 'ghost' ? styles.ghostField : ''}`}>
      <button
        type="button"
        ref={triggerRef}
        className={`${triggerClass[variant]} ${isOpen ? styles.open : ''} ${value ? '' : styles.placeholderText}`}
        autoFocus={autoFocus}
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        onClick={() => setIsOpen((open) => !open)}>
        {displayValue || placeholder || 'Select date'}
      </button>
      <Popover
        id="date-picker"
        isOpen={isOpen}
        triggerRef={triggerRef}
        width={282}
        onClose={close}
        className={styles.datePopoverContent}>
        <MiniCalendar
          value={value}
          onSelect={(iso) => {
            onClick(iso, iso);
            close();
          }}
        />
      </Popover>
    </div>
  );
}
