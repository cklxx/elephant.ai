'use client';

import { useState, useEffect, useRef } from 'react';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
}

export function Dialog({ open, onOpenChange, children }: DialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && open) {
        onOpenChange(false);
      }
    };

    const handleClickOutside = (e: MouseEvent) => {
      if (dialogRef.current && !dialogRef.current.contains(e.target as Node)) {
        onOpenChange(false);
      }
    };

    if (open) {
      document.addEventListener('keydown', handleEscape);
      document.addEventListener('mousedown', handleClickOutside);
      document.body.style.overflow = 'hidden';
    }

    return () => {
      document.removeEventListener('keydown', handleEscape);
      document.removeEventListener('mousedown', handleClickOutside);
      document.body.style.overflow = 'unset';
    };
  }, [open, onOpenChange]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center animate-fadeIn"
      role="dialog"
      aria-modal="true"
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />

      {/* Dialog content */}
      <div
        ref={dialogRef}
        className="relative z-10 animate-scaleIn"
      >
        {children}
      </div>
    </div>
  );
}

export interface DialogContentProps {
  className?: string;
  children: React.ReactNode;
  onClose?: () => void;
  showCloseButton?: boolean;
}

export function DialogContent({
  className,
  children,
  onClose,
  showCloseButton = true
}: DialogContentProps) {
  return (
    <div className={cn(
      'glass-card p-6 rounded-2xl shadow-strong max-w-lg w-full mx-4',
      'border border-gray-200/50',
      className
    )}>
      {showCloseButton && onClose && (
        <button
          onClick={onClose}
          className="absolute top-4 right-4 p-2 rounded-lg hover:bg-gray-100 transition-colors"
          aria-label="Close dialog"
        >
          <X className="h-5 w-5 text-gray-500" />
        </button>
      )}
      {children}
    </div>
  );
}

export function DialogHeader({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('mb-4', className)}>
      {children}
    </div>
  );
}

export function DialogTitle({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <h2 className={cn('text-2xl font-bold gradient-text', className)}>
      {children}
    </h2>
  );
}

export function DialogDescription({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <p className={cn('mt-2 text-sm text-gray-600', className)}>
      {children}
    </p>
  );
}

export function DialogFooter({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('mt-6 flex items-center justify-end gap-3', className)}>
      {children}
    </div>
  );
}

// Confirmation dialog hook
export function useConfirmDialog() {
  const [isOpen, setIsOpen] = useState(false);
  const [config, setConfig] = useState<{
    title: string;
    description: string;
    confirmText?: string;
    cancelText?: string;
    onConfirm: () => void;
    onCancel: () => void;
    variant?: 'default' | 'danger';
  } | null>(null);
  const isHandlingCloseRef = useRef(false);

  const confirm = (options: {
    title: string;
    description: string;
    confirmText?: string;
    cancelText?: string;
    variant?: 'default' | 'danger';
  }): Promise<boolean> => {
    return new Promise((resolve) => {
      setConfig({
        ...options,
        onConfirm: () => {
          setIsOpen(false);
          resolve(true);
        },
        onCancel: () => {
          setIsOpen(false);
          resolve(false);
        },
      });
      setIsOpen(true);
    });
  };

  const ConfirmDialog = () => {
    if (!config) return null;

    const confirmButtonClass = config.variant === 'danger'
      ? 'bg-destructive hover:bg-destructive/90 text-destructive-foreground'
      : 'bg-primary hover:bg-primary/90 text-primary-foreground';

    const handleConfirm = () => {
      config.onConfirm();
    };

    const handleCancel = () => {
      config.onCancel();
    };

    return (
      <Dialog open={isOpen} onOpenChange={setIsOpen}>
        <DialogContent onClose={handleCancel}>
          <DialogHeader>
            <DialogTitle>{config.title}</DialogTitle>
            <DialogDescription>{config.description}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <button
              onClick={handleCancel}
              className="px-4 py-2 rounded-lg border border-primary/30 bg-background text-foreground hover:bg-primary/10 transition-colors"
            >
              {config.cancelText || 'Cancel'}
            </button>
            <button
              onClick={config.onConfirm}
              className={cn(
                'px-4 py-2 rounded-lg font-medium transition-colors',
                confirmButtonClass
              )}
            >
              {config.confirmText || 'Confirm'}
            </button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );
  };

  return { confirm, ConfirmDialog };
}
