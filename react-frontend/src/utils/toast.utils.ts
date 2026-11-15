import { createElement } from 'react';
import { errorText } from './errorText';
import { toast as heroToast } from '@heroui/react';
import {
  CheckCircleIcon,
  InfoIcon,
  WarningCircleIcon,
  WarningIcon,
  type Icon,
} from '@phosphor-icons/react';

/** HeroUI's toast variants; `ToastHost` styles each one. */
export type ToastVariant = 'default' | 'accent' | 'success' | 'warning' | 'danger';

interface ToastOptions {
  description?: string;
  timeout?: number;
}

const icons: Record<ToastVariant, Icon> = {
  default: InfoIcon,
  accent: InfoIcon,
  success: CheckCircleIcon,
  warning: WarningIcon,
  danger: WarningCircleIcon,
};

// errors stay up longer so there's time to read what went wrong
const timeouts: Record<ToastVariant, number> = {
  default: 3500,
  accent: 3500,
  success: 3000,
  warning: 5000,
  danger: 6000,
};

function showToast(title: string, variant: ToastVariant, options?: ToastOptions) {
  heroToast(title, {
    variant,
    description: options?.description,
    timeout: options?.timeout ?? timeouts[variant],
    indicator: createElement(icons[variant], { size: 18, weight: 'fill' }),
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

  info: (title: string, options?: ToastOptions) => showToast(title, 'accent', options),

  show: (title: string, options?: ToastOptions) => showToast(title, 'default', options),
};
