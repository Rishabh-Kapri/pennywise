import {
  ArrowDownIcon as ArrowDown,
  ArrowUpIcon as ArrowUp,
  MoneyIcon as Banknote,
  CalendarDotsIcon as CalendarDays,
  FunnelSimpleIcon as Funnel,
  PlusIcon as Plus,
  MagnifyingGlassIcon as Search,
} from '@phosphor-icons/react';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import styles from './TransactionHeader.module.css';

export type MobileFilter = 'all' | 'incoming' | 'outgoing' | 'week';

export interface TransactionHeaderProps {
  name: string;
  balance: number;
  onTxnAdd: () => void;
  searchTerm: string;
  onSearchChange: (v: string) => void;
  mobileFilter: MobileFilter;
  onMobileFilterChange: (v: MobileFilter) => void;
  /** Off when the host page already shows the account name and balance. */
  showSummary?: boolean;
  /** Renders a button that shows/hides a filter panel the host owns. */
  filterToggle?: { open: boolean; active: boolean; onToggle: () => void };
}

export function TransactionHeader({
  name,
  balance,
  onTxnAdd,
  searchTerm,
  onSearchChange,
  mobileFilter,
  onMobileFilterChange,
  showSummary = true,
  filterToggle,
}: TransactionHeaderProps) {
  return (
    <div className={styles.container}>
      {showSummary && (
        <>
          <h2 className={styles.title}>
            <Banknote size={28} />
            <span>{name}</span>
          </h2>
          <div className={balance < 0 ? `${styles.negative} ${styles.amount}` : styles.amount}>
            <h3>{getCurrencyLocaleString(balance)}</h3>
          </div>
        </>
      )}
      <div className={styles.actionContainer}>
        <button type="button" className={styles.addButton} onClick={onTxnAdd}>
          <Plus size={16} weight="bold" />
          <span>Add Expense</span>
        </button>
        <div className={styles.trailingActions}>
          {filterToggle && (
            <button
              type="button"
              className={`${styles.filterToggle} ${filterToggle.open ? styles.filterToggleOpen : ''}`}
              aria-expanded={filterToggle.open}
              onClick={filterToggle.onToggle}>
              <Funnel size={16} weight={filterToggle.active ? 'bold' : 'regular'} />
              <span>Filters</span>
              {filterToggle.active && <span className={styles.filterDot} aria-label="Filters active" />}
            </button>
          )}
          <div className={styles.searchContainer}>
            <Search size={16} />
            <input
              type="search"
              aria-label="Search transactions"
              className={styles.searchInput}
              placeholder="Search transactions"
              value={searchTerm}
              onChange={(e) => onSearchChange(e.target.value)}
            />
          </div>
        </div>
      </div>
      <div className={styles.mobileFilterChips}>
        {(['incoming', 'outgoing', 'week'] as const).map((f) => {
          const Icon = f === 'incoming' ? ArrowDown : f === 'outgoing' ? ArrowUp : CalendarDays;
          const label = f === 'week' ? 'This week' : f.charAt(0).toUpperCase() + f.slice(1);
          const isSelected = mobileFilter === f;

          return (
            <button
              key={f}
              type="button"
              className={`${styles.filterChip} ${isSelected ? styles.activeFilterChip : ''}`}
              onClick={() => onMobileFilterChange(mobileFilter === f ? 'all' : f)}>
              <Icon size={16} weight={isSelected ? 'fill' : 'regular'} />
              <span>{label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
