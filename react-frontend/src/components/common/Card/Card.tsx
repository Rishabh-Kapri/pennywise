import { useId, type ReactNode } from 'react';
import styles from './Card.module.css';

interface CardProps {
  title: string;
  icon?: ReactNode;
  /** Right side of the header: a range toggle, a link. */
  actions?: ReactNode;
  /** Dims the card while a refetch is in flight; the old figures stay put. */
  refreshing?: boolean;
  className?: string;
  children: ReactNode;
}

/** The floating card the dashboard and report widgets sit in. */
export function Card({
  title,
  icon,
  actions,
  refreshing = false,
  className = '',
  children,
}: CardProps) {
  const headingId = useId();

  return (
    <section className={`${styles.card} ${className}`} aria-labelledby={headingId} aria-busy={refreshing}>
      <header className={styles.header}>
        <h2 id={headingId} className={styles.title}>
          {icon}
          {title}
        </h2>
        {actions && <div className={styles.actions}>{actions}</div>}
      </header>
      {children}
    </section>
  );
}
