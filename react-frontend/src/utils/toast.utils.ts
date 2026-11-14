import { createElement } from 'react';
import { errorText } from './errorText';
import { addToast } from '@heroui/react';
import {
  CheckCircleIcon,
  InfoIcon,
  WarningCircleIcon,
  WarningIcon,
  type Icon,
} from '@phosphor-icons/react';
import styles from './toast.module.css';

type ToastColor = 'default' | 'primary' | 'success' | 'warning' | 'danger';

interface ToastOptions {
  description?: string;
  timeout?: number;
}

const icons: Record<ToastColor, Icon> = {
  default: InfoIcon,
  primary: InfoIcon,
  success: CheckCircleIcon,
  warning: WarningIcon,
  danger: WarningCircleIcon,
};

// errors stay up longer so there's time to read what went wrong
const timeouts: Record<ToastColor, number> = {
  default: 3500,
  primary: 3500,
  success: 3000,
  warning: 5000,
  danger: 6000,
};

function showToast(title: string, color: ToastColor, options?: ToastOptions) {
  addToast({
    title,
    color,
    description: options?.description,
    timeout: options?.timeout ?? timeouts[color],
    variant: 'flat',
    shouldShowTimeoutProgress: true,
    // HeroUI clones this element and puts classNames.icon on it
    icon: createElement('span', null, createElement(icons[color], { size: 18, weight: 'fill' })),
    classNames: {
      base: `${styles.toast} ${styles[color]}`,
      icon: styles.icon,
      title: styles.title,
      description: styles.description,
      progressIndicator: styles.progress,
      motionDiv: styles.motion,
      closeButton: styles.close,
    },
  });
}

/**
 * Message from a thrown error or a rejected thunk, else the fallback. Server
 * codes ("INVALID_ARGUMENT: file is not a PDF") are put into words.
 */
export function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return errorText(error.message);
  if (typeof error === 'string' && error) return errorText(error);
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string' && error.message) {
    return errorText(error.message);
  }
  return fallback;
}

export const toast = {
  success: (title: string, options?: ToastOptions) => showToast(title, 'success', options),

  error: (title: string, options?: ToastOptions) => showToast(title, 'danger', options),

  warning: (title: string, options?: ToastOptions) => showToast(title, 'warning', options),

  info: (title: string, options?: ToastOptions) => showToast(title, 'primary', options),

  show: (title: string, options?: ToastOptions) => showToast(title, 'default', options),
};
