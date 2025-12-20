import { useEffect } from 'react';
import { useHeader } from '../../../context/HeaderContext';
import styles from './Dashboard.module.css';
import { store } from '@/app';

const DashboardHeaderContent = () => (
  <div>
    <div>Hi, Rishabh</div>
    <div></div>
  </div>
);

export default function Dashboard() {
  const { setHeaderContent } = useHeader();

  useEffect(() => {
    setHeaderContent(<DashboardHeaderContent />);

    // clear header content on unmount
    return () => setHeaderContent(null);
  }, [setHeaderContent]);

  useEffect(() => {
    const unsubscribe = store.subscribe(() => {
      console.log('State received:', store.getState());
    });
    return () => unsubscribe();
  }, []);

  return (
    <div className={styles.container}>
      <h1>Dashboard</h1>
    </div>
  );
}
