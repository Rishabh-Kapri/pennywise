import styles from './AppLoader.module.css';

interface AppLoaderProps {
  /** Fill the whole window (app start, sign-in check) instead of the page area. */
  fullScreen?: boolean;
  /** Shown under the mark; also what screen readers hear. */
  label?: string;
}

/**
 * The Pennywise mark drawing itself: each layer's outline is traced, then
 * fills in, then the mark fades and starts again.
 * Use it where the app waits before it has anything to draw; a page that
 * knows its shape uses `Skeleton` instead.
 */
export function AppLoader({ fullScreen = false, label = 'Loading' }: AppLoaderProps) {
  return (
    <div
      className={`${styles.loader} ${fullScreen ? styles.fullScreen : ''}`}
      role="status"
      aria-live="polite">
      <svg className={styles.mark} viewBox="96 96 320 320" aria-hidden="true">
        <path className={`${styles.layer} ${styles.top}`} pathLength={100} d="M164 154V142A30 30 0 0 1 194 112H318A30 30 0 0 1 348 142V154Z" />
        <path className={`${styles.layer} ${styles.middle}`} pathLength={100} d="M136 218V202A34 34 0 0 1 170 168H342A34 34 0 0 1 376 202V218Z" />
        <path
          className={`${styles.layer} ${styles.base}`}
          pathLength={100}
          fillRule="evenodd"
          d="M152 232H360A40 40 0 0 1 400 272V360A40 40 0 0 1 360 400H152A40 40 0 0 1 112 360V272A40 40 0 0 1 152 232ZM218 304A12 12 0 0 0 218 328H294A12 12 0 0 0 294 304Z"
        />
      </svg>
      <span className={styles.label}>{label}</span>
    </div>
  );
}

export default AppLoader;
