import { NavLink } from 'react-router-dom';
import { ChartPie, Wallet as WalletCards, Money as Banknote, List as Menu, X, Receipt as ReceiptIndianRupee, Users as UsersRound } from '@phosphor-icons/react';
import type { IconProps } from '@phosphor-icons/react';
import styles from './Navbar.module.css';
import { cloneElement, useState, useEffect, type ReactElement } from 'react';
import { useLocation } from 'react-router-dom';
import BudgetSwitcher from '@/features/budget/components/BudgetSwitcher';
import { UserMenu } from './UserMenu';

type NavIcon = ReactElement<IconProps>;

const NAV_ITEMS: {
  path: string;
  key: string;
  label: string;
  icon: NavIcon;
  exact: boolean;
}[] = [
  { path: '/dashboard', key: 'home', label: 'Home', icon: <ChartPie size={16} strokeWidth={1.75} />, exact: true },
  { path: '/transactions', key: 'transactions', label: 'Transactions', icon: <ReceiptIndianRupee size={16} strokeWidth={1.75} />, exact: false },
  { path: '/budget', key: 'budget', label: 'Budget', icon: <WalletCards size={16} strokeWidth={1.75} />, exact: false },
  { path: '/loans', key: 'loans', label: 'Loans', icon: <Banknote size={16} strokeWidth={1.75} />, exact: false },
  { path: '/payees', key: 'payees', label: 'Payees', icon: <UsersRound size={16} strokeWidth={1.75} />, exact: false },
];

function renderNavIcon(icon: NavIcon, isActive: boolean) {
  return cloneElement(icon, { weight: isActive ? 'fill' : 'regular' });
}

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
              {({ isActive }) => (
                <>
                  {renderNavIcon(item.icon, isActive)}
                  <span>{item.label}</span>
                </>
              )}
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

      {/* Mobile drawer — minimal: BudgetSwitcher + UserMenu moved here on open */}
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
            {({ isActive }) => (
              <>
                {renderNavIcon(item.icon, isActive)}
                <span>{item.label}</span>
              </>
            )}
          </NavLink>
        ))}
      </div>

      {/* Bottom mobile nav bar */}
      <nav className={styles.bottomNav}>
        {NAV_ITEMS.slice(0, 4).map((item) => (
          <NavLink
            key={item.key}
            to={item.path}
            end={item.exact}
            className={({ isActive }) =>
              `${styles.bottomNavItem} ${isActive ? styles.bottomNavItemActive : ''}`
            }>
            {({ isActive }) => (
              <>
                {renderNavIcon(item.icon, isActive)}
                <span>{item.label}</span>
              </>
            )}
          </NavLink>
        ))}
      </nav>
    </>
  );
}
