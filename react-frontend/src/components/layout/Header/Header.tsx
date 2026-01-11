import { useHeader } from '../../../context/HeaderContext';
import styles from './Header.module.css';

// dynamic header component
export default function Header() {
  const { headerContent } = useHeader();

  return <header className={styles.header}>{headerContent}</header>;
}
