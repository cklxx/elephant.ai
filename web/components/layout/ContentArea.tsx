'use client';

import { ReactNode, forwardRef } from 'react';
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
      <div
        ref={ref}
        className={cn(
          'flex-1 overflow-y-auto bg-gray-50/50',
          'console-scrollbar',
          className
        )}
      >
        {isEmpty ? (
          <div className="flex h-full items-center justify-center">
            {emptyState}
          </div>
        ) : (
          <div className="px-6 py-6">
            {children}
          </div>
        )}
      </div>
    );
  }
);

ContentArea.displayName = 'ContentArea';
