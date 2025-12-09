'use client';

import { ReactNode, forwardRef } from 'react';

import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';

interface ContentAreaProps {
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  emptyState?: ReactNode;
  isEmpty?: boolean;
  fullWidth?: boolean;
}

export const ContentArea = forwardRef<HTMLDivElement, ContentAreaProps>(
  (
    {
      children,
      className,
      contentClassName,
      emptyState,
      isEmpty = false,
      fullWidth = false,
    },
    ref
  ) => {
    const containerClasses = cn(
      'flex h-full w-full flex-col gap-4 px-4 py-6 sm:px-6',
      fullWidth ? 'max-w-none' : 'mx-auto max-w-5xl',
      contentClassName
    );

    return (
      <ScrollArea
        ref={ref}
        className={cn('flex-1', className)}
        viewportClassName="h-full"
      >
        {isEmpty ? (
          <div className="flex h-full items-center justify-center px-6 py-10">
            {emptyState}
          </div>
        ) : (
          <div className={containerClasses}>{children}</div>
        )}
      </ScrollArea>
    );
  }
);

ContentArea.displayName = 'ContentArea';
