import type { CSSProperties } from 'react';
import { Toast } from '@heroui/react';
import type { ToastVariant } from '@/utils/toast.utils';
import styles from './ToastHost.module.css';

/**
 * Where `toast.*` (utils/toast.utils) shows up. Mounted once at the root.
 * Renders each toast itself so it carries the app's card style and a
 * countdown bar instead of HeroUI's default look.
 */
export default function ToastHost() {
  return (
    // bottom-centre keeps toasts clear of the Ask Penny launcher and side-panel buttons
    <Toast.Provider placement="bottom">
      {({ toast }) => {
        const { title, description, indicator } = toast.content;
        const variant: ToastVariant = toast.content.variant ?? 'default';
        const countdown = { '--toast-timeout': `${toast.timeout ?? 0}ms` } as CSSProperties;
        return (
          <Toast toast={toast} variant={variant} className={`${styles.toast} ${styles[variant]}`}>
            <Toast.Indicator className={styles.icon}>{indicator}</Toast.Indicator>
            <Toast.Content>
              {title && <Toast.Title className={styles.title}>{title}</Toast.Title>}
              {description && <Toast.Description className={styles.description}>{description}</Toast.Description>}
            </Toast.Content>
            <Toast.CloseButton className={styles.close} />
            {toast.timeout ? <span aria-hidden className={styles.progress} style={countdown} /> : null}
          </Toast>
        );
      }}
    </Toast.Provider>
  );
}
