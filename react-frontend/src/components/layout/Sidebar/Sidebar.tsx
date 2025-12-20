import { NavLink } from 'react-router-dom';
import styles from './Sidebar.module.css';
import {
  ChartPie,
  CircleDollarSign,
  FileText,
  Landmark,
  PiggyBank,
  WalletCards,
  Lock,
} from 'lucide-react';
import { useAppSelector } from '@/app/hooks';
import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useState,
  type JSX,
} from 'react';
import { getCurrencyLocaleString } from '@/utils/date.utils';

interface NavItem {
  path: string;
  key: string;
  label: string;
  icon?: JSX.Element;
  meta?: {
    balance: string;
  };
  children?: NavItem[];
  isCollapsed?: boolean;
}

export default function Sidebar() {
  const navItems: NavItem[] = useMemo(
    () => [
      { path: '/', key: 'home', label: 'Overview', icon: <ChartPie /> },
      {
        path: '/budget',
        key: 'budget',
        label: 'Budget',
        icon: <WalletCards />,
      },
      {
        path: '/reports',
        key: 'reports',
        label: 'Reports',
        icon: <FileText />,
      },
      {
        path: '/transactions',
        key: 'all-transactions',
        label: 'All Accounts',
        icon: <Landmark />,
      },
    ],
    [],
  );
  const [dynamicNavItems, setDynamicNavItems] = useState<NavItem[]>([]);

  const { trackingAccounts, budgetAccounts, allAccounts } = useAppSelector(
    (state) => state.accounts,
  );

  const getNavItem = useCallback(
    (
      path: string,
      key: string,
      label: string,
      meta = { balance: '0' },
      isCollapsed?: boolean,
      icon?: JSX.Element,
    ): NavItem => {
      return {
        path,
        key,
        label,
        icon,
        meta,
        isCollapsed,
      };
    },
    [],
  );

  useEffect(() => {
    const newNavItems: NavItem[] = [];
    if (budgetAccounts.length > 0) {
      const navItem = getNavItem(
        '/accounts',
        'budget-accounts',
        'Budget Accounts',
        {
          balance: getCurrencyLocaleString(
            budgetAccounts.reduce((a, b) => a + (b.balance ?? 0), 0),
          ),
        },
        false,
        <CircleDollarSign />,
      );
      navItem.children = budgetAccounts.map((acc) => ({
        path: '/transactions/' + acc.id,
        key: 'budget-account-' + acc.id,
        label: acc.name,
        meta: {
          balance: getCurrencyLocaleString(acc.balance ?? 0),
        },
      }));
      navItem.children = budgetAccounts.map((acc) =>
        getNavItem(
          `/transactions/${acc.id}`,
          `budget-account-${acc.id}`,
          acc.name,
          { balance: getCurrencyLocaleString(acc.balance ?? 0) },
        ),
      );
      newNavItems.push(navItem);
    }
    if (trackingAccounts.length > 0) {
      const navItem = getNavItem(
        '',
        'tracking-transactions',
        'Tracking Accounts',
        {
          balance: getCurrencyLocaleString(
            trackingAccounts.reduce((a, b) => a + (b?.balance ?? 0), 0),
          ),
        },
        false,
        <PiggyBank />,
      );
      navItem.children = trackingAccounts.map((acc) =>
        getNavItem(
          '/transactions/' + acc.id,
          'tracking-account-' + acc.id,
          acc.name,
          { balance: getCurrencyLocaleString(acc.balance ?? 0) },
        ),
      );
      newNavItems.push(navItem);
    }
    if (allAccounts.length) {
      const navItem: NavItem = getNavItem(
        '',
        'closed',
        'Closed Accounts',
        { balance: getCurrencyLocaleString(0) },
        true,
        <Lock />,
      );
      navItem.children = allAccounts
        .filter((acc) => acc.closed)
        .map((acc) =>
          getNavItem(
            `/transactions/${acc.id}`,
            `closed-account-${acc.id}`,
            acc.name,
            { balance: getCurrencyLocaleString(acc.balance ?? 0) },
          ),
        );
      newNavItems.push(navItem);
    }
    setDynamicNavItems([...newNavItems]);
  }, [trackingAccounts, budgetAccounts, allAccounts, getNavItem]);

  const handleCollapse = (key: string) => {
    setDynamicNavItems((prev) =>
      prev.map((item) =>
        item.key === key
          ? { ...item, isCollapsed: !item.isCollapsed }
          : { ...item },
      ),
    );
  };

  return (
    <aside className={styles.sidebar}>
      <div className={styles.logo}>
        <h2>Pennywise</h2>
      </div>

      <nav className={styles.nav}>
        {navItems.map((item) => (
          <NavLink
            key={item.key}
            to={item.path}
            className={({ isActive }) =>
              isActive ? `${styles.active} ${styles.navItem}` : styles.navItem
            }>
            {item.icon && item.icon}
            <span className={styles.label}>{item.label}</span>
            {item.meta && (
              <span className={styles.meta}>{item.meta.balance}</span>
            )}
          </NavLink>
        ))}
        {dynamicNavItems.map((item) => (
          <Fragment key={item.key}>
            <div
              className={styles.dynamicItem}
              onClick={() => handleCollapse(item.key)}>
              {item?.icon && item?.icon}
              <span>{item.label}</span>
              {item?.meta && (
                <span className={styles.meta}>{item.meta.balance}</span>
              )}
            </div>
            {!item.isCollapsed && (
              <div className={styles.childContainer}>
                {item?.children?.map((child) => (
                  <NavLink
                    key={child.key}
                    to={child.path}
                    className={({ isActive }) =>
                      isActive
                        ? `${styles.navItem} ${styles.active}`
                        : styles.navItem
                    }>
                    {child.icon && child.icon}
                    <span className={styles.label}>{child.label}</span>
                    {child.meta && (
                      <span className={styles.meta}>{child.meta.balance}</span>
                    )}
                  </NavLink>
                ))}
              </div>
            )}
          </Fragment>
        ))}
      </nav>
    </aside>
  );
}
