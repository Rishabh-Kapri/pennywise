import { NavLink } from 'react-router-dom';
import { ChartPie, WalletCards, Banknote, Menu, X, ReceiptIndianRupee } from 'lucide-react';
import styles from './Navbar.module.css';
import { useState, useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import BudgetSwitcher from '@/features/budget/components/BudgetSwitcher';
import { UserMenu } from './UserMenu';

const NAV_ITEMS = [
  { path: '/', key: 'home', label: 'Home', icon: <ChartPie size={16} strokeWidth={1.75} />, exact: true },
  { path: '/transactions', key: 'transactions', label: 'Transactions', icon: <ReceiptIndianRupee size={16} strokeWidth={1.75} />, exact: false },
  { path: '/budget', key: 'budget', label: 'Budget', icon: <WalletCards size={16} strokeWidth={1.75} />, exact: false },
  { path: '/loans', key: 'loans', label: 'Loans', icon: <Banknote size={16} strokeWidth={1.75} />, exact: false },
];

export function Navbar() {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const location = useLocation();

  useEffect(() => {
    setIsMobileMenuOpen(false);
  }, [location.pathname]);

  return (
    <>
      <nav className={styles.navbar}>
        {/* Desktop nav tabs */}
        <div className={styles.navTabs}>
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.key}
              to={item.path}
              end={item.exact}
              className={({ isActive }) =>
                `${styles.navTab} ${isActive ? styles.navTabActive : ''}`
              }>
              {item.icon}
              <span>{item.label}</span>
            </NavLink>
          ))}
        </div>

        {/* Profile + mobile toggle */}
        <div className={styles.navRight}>
          <BudgetSwitcher />
          <UserMenu />
          <button
            type="button"
            className={styles.mobileMenuButton}
            aria-label={isMobileMenuOpen ? 'Close menu' : 'Open menu'}
            onClick={() => setIsMobileMenuOpen((v) => !v)}>
            {isMobileMenuOpen ? <X size={20} /> : <Menu size={20} />}
          </button>
        </div>
      </nav>

      {/* Mobile drawer */}
      {isMobileMenuOpen && (
        <button
          type="button"
          className={styles.backdrop}
          aria-label="Close menu"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      )}
      <div className={`${styles.mobileDrawer} ${isMobileMenuOpen ? styles.mobileDrawerOpen : ''}`}>
        {NAV_ITEMS.map((item) => (
          <NavLink
            key={item.key}
            to={item.path}
            end={item.exact}
            className={({ isActive }) =>
              `${styles.mobileNavItem} ${isActive ? styles.mobileNavItemActive : ''}`
            }>
            {item.icon}
            <span>{item.label}</span>
          </NavLink>
        ))}
      </div>
    </>
  );
}
