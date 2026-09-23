import { Moon, Sun } from '@phosphor-icons/react';
import { useTheme } from '@/context/ThemeContext';
import styles from './ThemeToggle.module.css';

export function ThemeToggle() {
  const { theme, toggleTheme } = useTheme();
  const nextTheme = theme === 'dark' ? 'light' : 'dark';
  return (
    <button
      type="button"
      className={styles.toggle}
      onClick={toggleTheme}
      aria-label={`Switch to ${nextTheme} theme`}
      title={`Switch to ${nextTheme} theme`}
    >
      <span className={styles.icon} data-theme-icon={nextTheme} aria-hidden="true">
        <Sun className={styles.sun} size={20} />
        <Moon className={styles.moon} size={20} />
      </span>
      <span className={styles.label}>{theme === 'dark' ? 'Light' : 'Dark'}</span>
    </button>
  );
}
