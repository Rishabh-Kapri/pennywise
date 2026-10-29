import { formatDigits, formatSignedCurrency } from '../../utils';
import styles from './Amount.module.css';

interface AmountProps {
  value: number;
  /** Prefix positive values with +. */
  signed?: boolean;
  className?: string;
}

/**
 * A headline figure: the rupee sign set small and raised, so the digits
 * carry the weight. Size it with the parent's font-size.
 */
export default function Amount({ value, signed = false, className = '' }: AmountProps) {
  const rounded = Math.round(value);
  const sign = rounded < 0 ? '−' : signed && rounded > 0 ? '+' : '';

  return (
    <span className={`${styles.amount} ${className}`}>
      <span className={styles.srOnly}>{formatSignedCurrency(rounded, signed)}</span>
      <span className={styles.visual} aria-hidden="true">
        {sign && <span className={styles.sign}>{sign}</span>}
        <span className={styles.currency}>₹</span>
        {formatDigits(rounded)}
      </span>
    </span>
  );
}
