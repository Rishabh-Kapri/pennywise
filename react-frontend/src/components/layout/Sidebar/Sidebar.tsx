import { NavLink, useLocation } from 'react-router-dom';
import styles from './Sidebar.module.css';
import { Money as Banknote, ChartPie, CurrencyCircleDollar as CircleDollarSign, FileText, Bank as Landmark, PiggyBank, Wallet as WalletCards, Lock, SidebarSimple as PanelLeftClose } from '@phosphor-icons/react';
import type { IconProps } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import {
  cloneElement,
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactElement,
} from 'react';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import { Tooltip } from '@heroui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '@heroui/popover';

type IconElement = ReactElement<IconProps>;

interface NavItem {
  path: string;
  key: string;
  label: string;
  icon?: IconElement;
  meta?: {
    balance: string;
  };
  children?: NavItem[];
  isCollapsed?: boolean;
}

interface SidebarProps {
  isMobileOpen?: boolean;
  onNavigate?: () => void;
}

function renderIcon(icon: IconElement | undefined, isSelected: boolean) {
  if (!icon) {
    return null;
  }

  return cloneElement(icon, { weight: isSelected ? 'fill' : 'regular' });
}

export default function Sidebar({ isMobileOpen = false, onNavigate }: SidebarProps) {
  const location = useLocation();
  const navItems: NavItem[] = useMemo(
    () => [
      {
        path: '/dashboard',
        key: 'home',
        label: 'Overview',
        icon: <ChartPie strokeWidth={1.5} />,
      },
      {
        path: '/budget',
        key: 'budget',
        label: 'Budget',
        icon: <WalletCards strokeWidth={1.5} />,
      },
      {
        path: '/reports',
        key: 'reports',
        label: 'Reports',
        icon: <FileText strokeWidth={1.5} />,
      },
      {
        path: '/transactions',
        key: 'all-transactions',
        label: 'All Accounts',
        icon: <Landmark strokeWidth={1.5} />,
      },
    ],
    [],
  );
  const [dynamicNavItems, setDynamicNavItems] = useState<NavItem[]>([]);
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [hoveredItemKey, setHoveredItemKey] = useState<string | null>(null);
  const isEffectivelyCollapsed = isCollapsed && !isMobileOpen;

  const { trackingAccounts, budgetAccounts, loanAccounts, allAccounts } = useAppSelector(
    (state) => state.accounts,
  );

  const getNavItem = useCallback(
    (
      path: string,
      key: string,
      label: string,
      meta = { balance: '0' },
      isCollapsed?: boolean,
      icon?: IconElement,
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
        <CircleDollarSign
          strokeWidth={1.5}
          height={isCollapsed ? 24 : 30}
          width={isCollapsed ? 24 : 30}
        />,
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
        <PiggyBank
          strokeWidth={1.5}
          height={isCollapsed ? 24 : 30}
          width={isCollapsed ? 24 : 30}
        />,
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
    if (loanAccounts.length > 0) {
      const navItem = getNavItem(
        '/loans',
        'loan-accounts',
        'Loan Accounts',
        {
          balance: getCurrencyLocaleString(
            loanAccounts.reduce((a, b) => a + Math.abs(b?.balance ?? 0), 0),
          ),
        },
        false,
        <Banknote
          strokeWidth={1.5}
          height={isCollapsed ? 24 : 30}
          width={isCollapsed ? 24 : 30}
        />,
      );
      navItem.children = loanAccounts.map((acc) =>
        getNavItem(
          `/loans/${acc.id}`,
          `loan-account-${acc.id}`,
          acc.name,
          { balance: getCurrencyLocaleString(Math.abs(acc.balance ?? 0)) },
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
        <Lock strokeWidth={1.5} size={24} />,
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
  }, [trackingAccounts, budgetAccounts, loanAccounts, allAccounts, isCollapsed, getNavItem]);

  const handleCollapse = (key: string) => {
    if (isCollapsed) {
      setIsCollapsed(false);
      setTimeout(() => {
        setDynamicNavItems((prev) =>
          prev.map((item) =>
            item.key === key
              ? { ...item, isCollapsed: !item.isCollapsed }
              : { ...item },
          ),
        );
      }, 100);
      return;
    }
    setDynamicNavItems((prev) =>
      prev.map((item) =>
        item.key === key
          ? { ...item, isCollapsed: !item.isCollapsed }
          : { ...item },
      ),
    );
  };

  const isDynamicItemSelected = (item: NavItem) => {
    if (
      item.path &&
      (location.pathname === item.path || location.pathname.startsWith(`${item.path}/`))
    ) {
      return true;
    }

    return item.children?.some((child) => child.path === location.pathname) ?? false;
  };

  return (
    <aside
      className={`${styles.sidebar} ${isEffectivelyCollapsed ? styles.collapsed : ''} ${
        isMobileOpen ? styles.open : ''
      }`}>
      <div className={styles.logo}>
        <h2>Pennywise</h2>
        <PanelLeftClose
          className={styles.toggleBtn}
          size={isCollapsed ? 24 : 20}
          strokeWidth={1.5}
          style={{
            transform: isCollapsed ? 'rotate(180deg)' : 'none',
            transition: 'transform 0.5s',
          }}
          onClick={() => setIsCollapsed(!isCollapsed)}
        />
      </div>

      <nav className={styles.nav}>
        {navItems.map((item) => (
          <Tooltip
            key={item.key}
            content={item.label}
            placement="right"
            isDisabled={!isEffectivelyCollapsed}
            classNames={{
              content: styles.tooltipContent,
            }}>
            <NavLink
              to={item.path}
              onClick={onNavigate}
              className={({ isActive }) =>
                isActive ? `${styles.active} ${styles.navItem}` : styles.navItem
              }>
              {({ isActive }) => (
                <>
                  {renderIcon(item.icon, isActive)}
                  <span className={styles.label}>{item.label}</span>
                  {item.meta && (
                    <span className={styles.meta}>{item.meta.balance}</span>
                  )}
                </>
              )}
            </NavLink>
          </Tooltip>
        ))}
        {dynamicNavItems.map((item) => {
          const isSelected = isDynamicItemSelected(item);

          return (
            <Fragment key={item.key}>
            {isEffectivelyCollapsed && item.children ? (
              <Popover
                placement="right"
                isOpen={hoveredItemKey === item.key}
                onOpenChange={(open) =>
                  setHoveredItemKey(open ? item.key : null)
                }>
                <PopoverTrigger>
                  <div
                    className={styles.dynamicItem}
                    onMouseEnter={() => setHoveredItemKey(item.key)}
                    onMouseLeave={() => setHoveredItemKey(null)}>
                    {renderIcon(item.icon, isSelected)}
                    <span>{item.label}</span>
                  </div>
                </PopoverTrigger>
                <PopoverContent
                  className={styles.popoverContent}
                  onMouseEnter={() => setHoveredItemKey(item.key)}>
                  <div className={styles.popoverHeader}>{item.label}</div>
                  {item.children.map((child) => (
                    <NavLink
                      key={child.key}
                      to={child.path}
                      onClick={onNavigate}
                      className={({ isActive }) =>
                        isActive
                          ? `${styles.navItem} ${styles.active}`
                          : styles.navItem
                      }>
                      <span className={`${styles.label} ${styles.truncate}`}>
                        {child.label}
                      </span>
                      {child.meta && (
                        <span className={`${styles.meta} ${styles.truncate}`}>
                          {child.meta.balance}
                        </span>
                      )}
                    </NavLink>
                  ))}
                </PopoverContent>
              </Popover>
            ) : (
              <Tooltip
                content={item.label}
                placement="right"
                isDisabled={!isEffectivelyCollapsed}
                classNames={{
                  content: styles.tooltipContent,
                }}>
                <div
                  className={styles.dynamicItem}
                  onClick={() => handleCollapse(item.key)}>
                  {renderIcon(item.icon, isSelected)}
                  <span>{item.label}</span>
                  {item?.meta && (
                    <span className={styles.meta}>{item.meta.balance}</span>
                  )}
                </div>
              </Tooltip>
            )}

            {!item.isCollapsed && !isEffectivelyCollapsed && (
              <div className={styles.childContainer}>
                {item?.children?.map((child) => (
                  <Tooltip
                    key={child.key}
                    content={child.label}
                    placement="right"
                    isDisabled={!isEffectivelyCollapsed}
                    classNames={{
                      content: styles.tooltipContent,
                    }}>
                    <NavLink
                      to={child.path}
                      onClick={onNavigate}
                      className={({ isActive }) =>
                        isActive
                          ? `${styles.navItem} ${styles.active}`
                          : styles.navItem
                      }>
                      {({ isActive }) => (
                        <>
                          {renderIcon(child.icon, isActive)}
                          <span className={`${styles.label} ${styles.truncate}`}>
                            {child.label}
                          </span>
                          {child.meta && (
                            <span className={`${styles.meta} ${styles.truncate}`}>
                              {child.meta.balance}
                            </span>
                          )}
                        </>
                      )}
                    </NavLink>
                  </Tooltip>
                ))}
              </div>
            )}
            </Fragment>
          );
        })}
      </nav>
    </aside>
  );
}
