import { addToast } from '@heroui/react';

type ToastColor = 'default' | 'primary' | 'secondary' | 'success' | 'warning' | 'danger';

interface ToastOptions {
  description?: string;
  timeout?: number;
  variant?: 'solid' | 'bordered' | 'flat';
}

function showToast(title: string, color: ToastColor, options?: ToastOptions) {
  addToast({
    title,
    color,
    description: options?.description,
    timeout: options?.timeout,
    variant: options?.variant ?? 'flat',
  });
}

export const toast = {
  success: (title: string, options?: ToastOptions) =>
    showToast(title, 'success', options),

  error: (title: string, options?: ToastOptions) =>
    showToast(title, 'danger', options),

  warning: (title: string, options?: ToastOptions) =>
    showToast(title, 'warning', options),

  info: (title: string, options?: ToastOptions) =>
    showToast(title, 'primary', options),

  show: (title: string, options?: ToastOptions) =>
    showToast(title, 'default', options),
};
