import { ArrowDown, ArrowUp, Money as Banknote, CalendarDots as CalendarDays, Plus, MagnifyingGlass as Search } from '@phosphor-icons/react';
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
}

export function TransactionHeader({
  name,
  balance,
  onTxnAdd,
  searchTerm,
  onSearchChange,
  mobileFilter,
  onMobileFilterChange,
}: TransactionHeaderProps) {
  return (
    <div className={styles.container}>
      <h2 className={styles.title}>
        <Banknote size={28} />
        <span>{name}</span>
      </h2>
      <div className={balance < 0 ? `${styles.negative} ${styles.amount}` : styles.amount}>
        <h3>{getCurrencyLocaleString(balance)}</h3>
      </div>
      <div className={styles.actionContainer}>
        <div className={styles.addButton} onClick={onTxnAdd}>
          <Plus size={16} />
          <span>Add Expense</span>
        </div>
        <div className={styles.searchContainer}>
          <Search size={16} />
          <input
            type="text"
            className={styles.searchInput}
            placeholder="Search transactions"
            value={searchTerm}
            onChange={(e) => onSearchChange(e.target.value)}
          />
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
