import { forwardRef } from 'react';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import styles from './CategoryGroup.module.css';

type AmountVariant = 'default' | 'balance';

interface AmountCellProps {
  value: number;
  variant?: AmountVariant;
  onClick?: (e: React.MouseEvent) => void;
  id?: string;
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
      id,
      balanceClassName,
      overspentClassName,
      ...ariaProps
    },
    ref,
  ) => {
    const isZero = value === 0;
    const isNegative = value < 0;
    const isBalance = variant === 'balance';

    const className = [
      isZero ? styles.noAmount : styles.amount,
      !balanceClassName && isBalance && styles.itemBalance,
      !overspentClassName && isBalance && isNegative && styles.itemOverspent,
      balanceClassName && isBalance && value > 0 && balanceClassName,
      overspentClassName && isNegative && overspentClassName,
    ]
      .filter(Boolean)
      .join(' ');

    return (
      <span
        ref={ref}
        id={id}
        className={className}
        onClick={onClick}
        {...ariaProps}>
        {getCurrencyLocaleString(value)}
      </span>
    );
  },
);

AmountCell.displayName = 'AmountCell';
