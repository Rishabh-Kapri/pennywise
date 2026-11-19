import { PennywiseMark } from '../PennywiseMark/PennywiseMark';
import styles from './AppLoader.module.css';

interface AppLoaderProps {
  /** Fill the whole window (app start, sign-in check) instead of the page area. */
  fullScreen?: boolean;
  /** Shown under the mark; also what screen readers hear. */
  label?: string;
}

/**
 * The Pennywise mark playing its loop: emails fall into the water and
 * become its lines, then the sun rises.
 * Use it where the app waits before it has anything to draw; a page that
 * knows its shape uses `Skeleton` instead.
 */
export function AppLoader({ fullScreen = false, label = 'Loading' }: AppLoaderProps) {
  return (
    <div
      className={`${styles.loader} ${fullScreen ? styles.fullScreen : ''}`}
      role="status"
      aria-live="polite">
      <PennywiseMark animated className={styles.mark} />
      <span className={styles.label}>{label}</span>
    </div>
  );
}

export default AppLoader;
