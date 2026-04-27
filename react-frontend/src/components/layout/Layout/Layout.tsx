import { Outlet } from 'react-router-dom';
import Header from '../Header/Header';
import { Navbar } from '../Navbar/Navbar';
import styles from './Layout.module.css';
import { HeaderProvider } from '../../../context/HeaderProvider';
import { SidePanelProvider } from '../../../context/SidePanelProvider';
import { useSidePanel } from '../../../context/SidePanelContext';
import { useEffect } from 'react';
import { fetchAllBudgets } from '@/features/budget';
import { useAppDispatch } from '@/app/hooks';

function MainWithSidePanel() {
  const { sidePanelContent } = useSidePanel();

  return (
    <div className={styles.mainWrapper}>
      <main className={styles.mainContent}>
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

  useEffect(() => {
    dispatch(fetchAllBudgets());
  }, [dispatch]);

  return (
    <div className={styles.layout}>
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
