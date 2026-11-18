import { Outlet, useLocation } from 'react-router-dom';
import { lazy, Suspense, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Navbar } from '../Navbar/Navbar';
import { LayoutSlotContentContext, LayoutSlotSettersContext } from '../slots';
import styles from './Layout.module.css';
import { fetchAllBudgets, selectAllBudgets, selectSelectedBudget } from '@/features/budget/store/budgetSlice';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import BudgetOnboarding from '@/features/budget/components/BudgetOnboarding';
import { WebSocketProvider } from '@/features/websocket/WebSocketProvider';
import { AgentChat } from '@/features/agent/components/AgentChat';
import { selectIsDemoUser } from '@/features/auth/store/authSlice';
import { selectOnboarding } from '@/features/onboarding/hooks/useOnboarding';
import { isOwner } from '@/features/sharing/types';

const OnboardingWizard = lazy(() => import('@/features/onboarding/components/OnboardingWizard'));

function LayoutSlotsProvider({ children }: { children: ReactNode }) {
  const [headerContent, setHeaderContent] = useState<ReactNode>(null);
  const [sidePanelContent, setSidePanelContent] = useState<ReactNode>(null);
  const setters = useMemo(() => ({ setHeaderContent, setSidePanelContent }), []);
  const content = useMemo(() => ({ headerContent, sidePanelContent }), [headerContent, sidePanelContent]);
  return (
    <LayoutSlotSettersContext.Provider value={setters}>
      <LayoutSlotContentContext.Provider value={content}>{children}</LayoutSlotContentContext.Provider>
    </LayoutSlotSettersContext.Provider>
  );
}

function Header() {
  const { headerContent } = useContext(LayoutSlotContentContext);
  return <header>{headerContent}</header>;
}

function MainWithSidePanel() {
  const { sidePanelContent } = useContext(LayoutSlotContentContext);
  const isDashboard = useLocation().pathname === '/dashboard';

  return (
    <div className={styles.mainWrapper}>
      <main className={`${styles.mainContent} ${isDashboard ? styles.dashboardMainContent : ''}`}>
        <Suspense fallback={<div>Loading...</div>}>
          <Outlet />
        </Suspense>
      </main>
      {sidePanelContent && <aside className={styles.sidePanel}>{sidePanelContent}</aside>}
      <AgentChat />
    </div>
  );
}

export default function Layout() {
  const dispatch = useAppDispatch();
  const budgets = useAppSelector(selectAllBudgets);
  const onboarding = useAppSelector(selectOnboarding);
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const selectedRole = useAppSelector(selectSelectedBudget)?.role;
  const [hasLoadedBudgets, setHasLoadedBudgets] = useState(false);

  useEffect(() => {
    dispatch(fetchAllBudgets()).finally(() => setHasLoadedBudgets(true));
  }, [dispatch]);

  if (hasLoadedBudgets && budgets.length === 0) {
    return <BudgetOnboarding />;
  }

  // A budget that hasn't finished setup opens the wizard instead of the app.
  // The websocket stays up so imported transactions arrive while it runs.
  // Only the owner runs setup: it writes AI keys and mailboxes, which a
  // member can't change. Someone who joined an unfinished budget gets the app.
  if (hasLoadedBudgets && onboarding && onboarding.step !== 'done' && !isDemoUser && isOwner(selectedRole)) {
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
      <LayoutSlotsProvider>
        <Header />
        <MainWithSidePanel />
      </LayoutSlotsProvider>
    </div>
  );
}
