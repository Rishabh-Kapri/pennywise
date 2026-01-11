import { Outlet } from 'react-router-dom';
import Header from '../Header/Header';
import Sidebar from '../Sidebar/Sidebar';
import styles from './Layout.module.css';
import { HeaderProvider } from '../../../context/HeaderProvider';
import { useEffect } from 'react';
import { fetchAllBudgets } from '@/features/budget';
import { useAppDispatch } from '@/app/hooks';

export default function Layout() {
  const dispatch = useAppDispatch();

  useEffect(() => {
    dispatch(fetchAllBudgets());
  }, [dispatch]);

  return (
    <div className={styles.layout}>
      <Sidebar />
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
