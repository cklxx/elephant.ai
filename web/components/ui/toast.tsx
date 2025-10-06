'use client';

import { type ReactNode } from 'react';
import { Toaster as SonnerToaster, toast as sonnerToast } from 'sonner';

export function Toaster() {
  return (
    <SonnerToaster
      position="top-right"
      expand={true}
      richColors
      closeButton
      duration={5000}
      toastOptions={{
        classNames: {
          toast: 'glass-card shadow-strong border-0',
          title: 'font-semibold text-gray-900',
          description: 'text-sm text-gray-600',
          actionButton: 'bg-blue-600 text-white hover:bg-blue-700',
          cancelButton: 'bg-gray-100 text-gray-700 hover:bg-gray-200',
          closeButton: 'hover:bg-gray-100',
        },
      }}
    />
  );
}

// Type-safe toast wrapper
export const toast = {
  success: (message: string, description?: string) => {
    return sonnerToast.success(message, { description });
  },
  error: (message: string, description?: string) => {
    return sonnerToast.error(message, {
      description,
      duration: Infinity, // Errors stay until dismissed
    });
  },
  info: (message: string, description?: string) => {
    return sonnerToast.info(message, { description });
  },
  warning: (message: string, description?: string) => {
    return sonnerToast.warning(message, { description });
  },
  promise: <T,>(
    promise: Promise<T>,
    options: {
      loading: string;
      success: string | ((data: T) => string);
      error: string | ((error: Error) => string);
    }
  ) => {
    return sonnerToast.promise(promise, options);
  },
  custom: (component: any) => {
    return sonnerToast.custom(component);
  },
  dismiss: (toastId?: string | number) => {
    return sonnerToast.dismiss(toastId);
  },
};
