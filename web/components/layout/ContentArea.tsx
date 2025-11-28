'use client';

import { ReactNode, forwardRef } from 'react';

import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';

interface ContentAreaProps {
  children: ReactNode;
  className?: string;
  emptyState?: ReactNode;
  isEmpty?: boolean;
}

export const ContentArea = forwardRef<HTMLDivElement, ContentAreaProps>(
  ({ children, className, emptyState, isEmpty = false }, ref) => {
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
          <div className="mx-auto flex h-full max-w-5xl flex-col gap-4 px-4 py-6 sm:px-6">
            {children}
          </div>
        )}
      </ScrollArea>
    );
  }
);

ContentArea.displayName = 'ContentArea';
