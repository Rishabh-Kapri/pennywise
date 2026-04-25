import { Outlet, useLocation } from 'react-router-dom';
import Header from '../Header/Header';
import Sidebar from '../Sidebar/Sidebar';
import styles from './Layout.module.css';
import { HeaderProvider } from '../../../context/HeaderProvider';
import { useEffect, useState } from 'react';
import { fetchAllBudgets } from '@/features/budget';
import { useAppDispatch } from '@/app/hooks';
import { Menu, X } from 'lucide-react';

export default function Layout() {
  const dispatch = useAppDispatch();
  const location = useLocation();
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  useEffect(() => {
    dispatch(fetchAllBudgets());
  }, [dispatch]);

  useEffect(() => {
    setIsSidebarOpen(false);
  }, [location.pathname]);

  return (
    <div className={styles.layout}>
      <button
        type="button"
        className={styles.mobileMenuButton}
        aria-label={isSidebarOpen ? 'Close navigation' : 'Open navigation'}
        aria-expanded={isSidebarOpen}
        onClick={() => setIsSidebarOpen((open) => !open)}>
        {isSidebarOpen ? <X size={22} /> : <Menu size={22} />}
      </button>
      {isSidebarOpen && (
        <button
          type="button"
          className={styles.backdrop}
          aria-label="Close navigation"
          onClick={() => setIsSidebarOpen(false)}
        />
      )}
      <Sidebar
        isMobileOpen={isSidebarOpen}
        onNavigate={() => setIsSidebarOpen(false)}
      />
      <div className={styles.mainWrapper}>
        <HeaderProvider>
          <Header />
          <main className={styles.mainContent}>
            <Outlet />
          </main>
        </HeaderProvider>
      </div>
    </div>
  );
}
