import { Outlet, useLocation } from 'react-router-dom';
import Header from '../Header/Header';
import { Navbar } from '../Navbar/Navbar';
import styles from './Layout.module.css';
import { HeaderProvider } from '../../../context/HeaderProvider';
import { SidePanelProvider } from '../../../context/SidePanelProvider';
import { useSidePanel } from '../../../context/SidePanelContext';
import { useEffect, useState } from 'react';
import {
  fetchAllBudgets,
  selectAllBudgets,
} from '@/features/budget';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import BudgetOnboarding from '@/features/budget/components/BudgetOnboarding';
import { WebSocketProvider } from '@/features/websocket/WebSocketProvider';

function MainWithSidePanel() {
  const { sidePanelContent } = useSidePanel();
  const location = useLocation();
  const isDashboard = location.pathname === '/dashboard' || location.pathname === '/';

  return (
    <div className={styles.mainWrapper}>
      <main className={`${styles.mainContent} ${isDashboard ? styles.dashboardMainContent : ''}`}>
        <Outlet />
      </main>
      {sidePanelContent && (
        <aside className={styles.sidePanel}>
          {sidePanelContent}
        </aside>
      )}
    </div>
  );
}

export default function Layout() {
  const dispatch = useAppDispatch();
  const budgets = useAppSelector(selectAllBudgets);
  const [hasLoadedBudgets, setHasLoadedBudgets] = useState(false);

  useEffect(() => {
    dispatch(fetchAllBudgets()).finally(() => setHasLoadedBudgets(true));
  }, [dispatch]);

  if (hasLoadedBudgets && budgets.length === 0) {
    return <BudgetOnboarding />;
  }

  return (
    <div className={styles.layout}>
      <WebSocketProvider />
      <Navbar />
      <HeaderProvider>
        <SidePanelProvider>
          <Header />
          <MainWithSidePanel />
        </SidePanelProvider>
      </HeaderProvider>
    </div>
  );
}
