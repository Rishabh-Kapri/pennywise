import { forwardRef } from 'react';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import styles from './AmountCell.module.css';

type AmountVariant = 'default' | 'balance';

interface AmountCellProps {
  value: string | number;
  variant?: AmountVariant;
  onClick?: (e: React.MouseEvent) => void;
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onBlur?: (e: React.FocusEvent) => void;
  onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  id?: string;
  isEditing?: boolean;
  balanceClassName?: string;
  overspentClassName?: string;
  'aria-haspopup'?: boolean | 'dialog' | 'menu' | 'listbox' | 'tree' | 'grid';
  'aria-controls'?: string;
}

export const AmountCell = forwardRef<HTMLSpanElement, AmountCellProps>(
  (
    {
      value,
      variant = 'default',
      onClick,
      onChange,
      onBlur,
      onKeyDown,
      id,
      balanceClassName,
      overspentClassName,
      isEditing,
      ...ariaProps
    },
    ref,
  ) => {
    const valueNum = Number(value);
    const isZero = valueNum === 0;
    const isNegative = valueNum < 0;
    const isBalance = variant === 'balance';

    const className = [
      isZero ? styles.noAmount : styles.amount,
      !balanceClassName && isBalance && styles.itemBalance,
      !overspentClassName && isBalance && isNegative && styles.itemOverspent,
      balanceClassName && isBalance && valueNum > 0 && balanceClassName,
      overspentClassName && isNegative && overspentClassName,
    ]
      .filter(Boolean)
      .join(' ');

    if (isEditing) {
      return (
        <input
          id={id}
          type="text"
          className={`${className} ${styles.input}`}
          {...ariaProps}
          onChange={onChange}
          onBlur={onBlur}
          onKeyDown={onKeyDown}
          onClick={(e) => e.stopPropagation()}
          value={value}
          inputMode="decimal"
          aria-label="Assigned amount"
          autoFocus
          onFocus={(e) => e.currentTarget.select()}
        />
      );
    }

    return (
      <span
        ref={ref}
        id={id}
        className={className}
        onClick={onClick}
        {...ariaProps}>
        {/* `|| 0` normalizes -0 so it never renders as "-₹0" */}
        {getCurrencyLocaleString(valueNum || 0, 'INR', 'en-IN', {
          minimumFractionDigits: 0,
          maximumFractionDigits: 2,
        })}
      </span>
    );
  },
);

AmountCell.displayName = 'AmountCell';
