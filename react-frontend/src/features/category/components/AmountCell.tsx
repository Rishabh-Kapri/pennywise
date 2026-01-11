import { forwardRef } from 'react';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import styles from './CategoryGroup.module.css';

type AmountVariant = 'default' | 'balance';

interface AmountCellProps {
  value: string | number;
  variant?: AmountVariant;
  onClick?: (e: React.MouseEvent) => void;
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onBlur?: (e: React.FocusEvent) => void;
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
          value={value}
          autoFocus
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
        {getCurrencyLocaleString(valueNum)}
      </span>
    );
  },
);

AmountCell.displayName = 'AmountCell';
