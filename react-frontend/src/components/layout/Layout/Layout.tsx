import { Outlet, useLocation } from 'react-router-dom';
import Header from '../Header/Header';
import { Navbar } from '../Navbar/Navbar';
import styles from './Layout.module.css';
import { HeaderProvider } from '../../../context/HeaderProvider';
import { SidePanelProvider } from '../../../context/SidePanelProvider';
import { useSidePanel } from '../../../context/SidePanelContext';
import { lazy, Suspense, useEffect, useState } from 'react';
import {
  fetchAllBudgets,
  selectAllBudgets,
} from '@/features/budget';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import BudgetOnboarding from '@/features/budget/components/BudgetOnboarding';
import { WebSocketProvider } from '@/features/websocket/WebSocketProvider';
import { AgentChat } from '@/components/common';
import { selectIsDemoUser } from '@/features/auth/store';
import { selectOnboarding } from '@/features/onboarding/hooks/useOnboarding';

const OnboardingWizard = lazy(() => import('@/features/onboarding/components/OnboardingWizard'));

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
      <AgentChat />
    </div>
  );
}

export default function Layout() {
  const dispatch = useAppDispatch();
  const budgets = useAppSelector(selectAllBudgets);
  const onboarding = useAppSelector(selectOnboarding);
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const [hasLoadedBudgets, setHasLoadedBudgets] = useState(false);

  useEffect(() => {
    dispatch(fetchAllBudgets()).finally(() => setHasLoadedBudgets(true));
  }, [dispatch]);

  if (hasLoadedBudgets && budgets.length === 0) {
    return <BudgetOnboarding />;
  }

  // A budget that hasn't finished setup opens the wizard instead of the app.
  // The websocket stays up so imported transactions arrive while it runs.
  if (hasLoadedBudgets && onboarding && onboarding.step !== 'done' && !isDemoUser) {
    return (
      <>
        <WebSocketProvider />
        <Suspense fallback={null}>
          <OnboardingWizard />
        </Suspense>
      </>
    );
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
