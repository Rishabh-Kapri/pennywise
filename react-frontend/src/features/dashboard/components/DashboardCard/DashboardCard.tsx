import { useId, type ReactNode } from 'react';
import styles from './DashboardCard.module.css';

interface DashboardCardProps {
  title: string;
  icon?: ReactNode;
  /** Right side of the header: a range toggle, a link. */
  actions?: ReactNode;
  /** Dims the card while a refetch is in flight; the old figures stay put. */
  refreshing?: boolean;
  className?: string;
  children: ReactNode;
}

/** The floating card every dashboard widget sits in. */
export default function DashboardCard({
  title,
  icon,
  actions,
  refreshing = false,
  className = '',
  children,
}: DashboardCardProps) {
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
