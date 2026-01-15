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
  PanelLeftClose,
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
import { Tooltip } from '@heroui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '@heroui/popover';

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

interface SidebarProps {
  isOpen?: boolean;
  onClose?: () => void;
}

export default function Sidebar({ isOpen = false, onClose }: SidebarProps) {
  const navItems: NavItem[] = useMemo(
    () => [
      {
        path: '/',
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

  // Force expand sidebar on mobile viewport
  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth <= 768) {
        setIsCollapsed(false);
      }
    };
    handleResize(); // Initial check
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

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
  }, [trackingAccounts, budgetAccounts, allAccounts, isCollapsed, getNavItem]);

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

  return (
    <aside
      className={`${styles.sidebar} ${isCollapsed ? styles.collapsed : ''} ${isOpen ? styles.mobileOpen : ''}`}>
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
            isDisabled={!isCollapsed}
            classNames={{
              content: styles.tooltipContent,
            }}>
            <NavLink
              to={item.path}
              onClick={onClose}
              className={({ isActive }) =>
                isActive ? `${styles.active} ${styles.navItem}` : styles.navItem
              }>
              {item.icon && item.icon}
              <span className={styles.label}>{item.label}</span>
              {item.meta && (
                <span className={styles.meta}>{item.meta.balance}</span>
              )}
            </NavLink>
          </Tooltip>
        ))}
        {dynamicNavItems.map((item) => (
          <Fragment key={item.key}>
            {isCollapsed && item.children ? (
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
                    {item?.icon && item?.icon}
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
                      onClick={onClose}
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
                isDisabled={!isCollapsed}
                classNames={{
                  content: styles.tooltipContent,
                }}>
                <div
                  className={styles.dynamicItem}
                  onClick={() => handleCollapse(item.key)}>
                  {item?.icon && item?.icon}
                  <span>{item.label}</span>
                  {item?.meta && (
                    <span className={styles.meta}>{item.meta.balance}</span>
                  )}
                </div>
              </Tooltip>
            )}

            {!item.isCollapsed && !isCollapsed && (
              <div className={styles.childContainer}>
                {item?.children?.map((child) => (
                  <Tooltip
                    key={child.key}
                    content={child.label}
                    placement="right"
                    isDisabled={!isCollapsed}
                    classNames={{
                      content: styles.tooltipContent,
                    }}>
                    <NavLink
                      to={child.path}
                      onClick={onClose}
                      className={({ isActive }) =>
                        isActive
                          ? `${styles.navItem} ${styles.active}`
                          : styles.navItem
                      }>
                      {child.icon && child.icon}
                      <span className={`${styles.label} ${styles.truncate}`}>
                        {child.label}
                      </span>
                      {child.meta && (
                        <span className={`${styles.meta} ${styles.truncate}`}>
                          {child.meta.balance}
                        </span>
                      )}
                    </NavLink>
                  </Tooltip>
                ))}
              </div>
            )}
          </Fragment>
        ))}
      </nav>
    </aside>
  );
}
