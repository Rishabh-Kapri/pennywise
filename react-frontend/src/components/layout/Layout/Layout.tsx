import { Outlet } from 'react-router-dom';
import Header from '../Header/Header';
import Sidebar from '../Sidebar/Sidebar';
import styles from './Layout.module.css';
import { HeaderProvider } from '../../../context/HeaderProvider';
import { useEffect, useState, useCallback } from 'react';
import { fetchAllBudgets } from '@/features/budget';
import { useAppDispatch } from '@/app/hooks';
import { Menu } from 'lucide-react';

export default function Layout() {
  const dispatch = useAppDispatch();
  const [isMobileSidebarOpen, setIsMobileSidebarOpen] = useState(false);

  useEffect(() => {
    dispatch(fetchAllBudgets());
  }, [dispatch]);

  // Close sidebar on window resize to desktop
  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth > 768) {
        setIsMobileSidebarOpen(false);
      }
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const handleCloseSidebar = useCallback(() => {
    setIsMobileSidebarOpen(false);
  }, []);

  return (
    <div className={styles.layout}>
      <div
        className={`${styles.overlay} ${isMobileSidebarOpen ? styles.visible : ''}`}
        onClick={handleCloseSidebar}
      />

      <Sidebar isOpen={isMobileSidebarOpen} onClose={handleCloseSidebar} />

      <div className={styles.mainWrapper}>
        <button
          className={styles.mobileMenuBtn}
          onClick={() => setIsMobileSidebarOpen(true)}
          aria-label="Open menu"
        >
          <Menu size={24} strokeWidth={1.5} />
        </button>
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
