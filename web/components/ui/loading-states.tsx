'use client';

import { Loader2, AlertCircle, CheckCircle2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Skeleton } from './skeleton';

export interface LoadingSpinnerProps {
  size?: 'sm' | 'md' | 'lg';
  className?: string;
  label?: string;
}

export function LoadingSpinner({ size = 'md', className, label }: LoadingSpinnerProps) {
  const sizeClasses = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12',
  };

  return (
    <div className={cn('flex flex-col items-center justify-center gap-3', className)}>
      <Loader2 className={cn('animate-spin-fast', sizeClasses[size])} />
      {label && <p>{label}</p>}
    </div>
  );
}

export interface LoadingStateProps {
  status: 'loading' | 'success' | 'error' | 'idle';
  loadingText?: string;
  successText?: string;
  errorText?: string;
  className?: string;
}

export function LoadingState({
  status,
  loadingText = 'Loading...',
  successText = 'Success!',
  errorText = 'Error occurred',
  className,
}: LoadingStateProps) {
  if (status === 'idle') return null;

  return (
    <div className={cn('flex items-center gap-3 p-4', className)}>
      {status === 'loading' && (
        <>
          <Loader2 className="h-5 w-5 animate-spin-fast" />
          <span>{loadingText}</span>
        </>
      )}
      {status === 'success' && (
        <>
          <CheckCircle2 className="h-5 w-5" />
          <span>{successText}</span>
        </>
      )}
      {status === 'error' && (
        <>
          <AlertCircle className="h-5 w-5" />
          <span>{errorText}</span>
        </>
      )}
    </div>
  );
}

export function LoadingOverlay({
  visible,
  message = 'Processing...',
  fullscreen = false,
}: {
  visible: boolean;
  message?: string;
  fullscreen?: boolean;
}) {
  if (!visible) return null;

  return (
    <div
      className={cn(
        'flex items-center justify-center z-50',
        fullscreen ? 'fixed inset-0' : 'absolute inset-0'
      )}
    >
      <div className="p-8">
        <LoadingSpinner size="lg" label={message} />
      </div>
    </div>
  );
}

export function PulsingDot({ color = 'primary' }: { color?: 'primary' | 'green' | 'red' | 'yellow' }) {
  const colorClasses = {
    primary: '',
    green: '',
    red: '',
    yellow: '',
  };

  return (
    <span className="relative flex h-3 w-3">
      <span
        className={cn(
          'animate-ping absolute inline-flex h-full w-full',
          colorClasses[color]
        )}
      ></span>
      <span className={cn('relative inline-flex h-3 w-3', colorClasses[color])}></span>
    </span>
  );
}

export function LoadingDots({
  count = 3,
  className,
  dotClassName,
}: {
  count?: number;
  className?: string;
  dotClassName?: string;
}) {
  return (
    <>
      <span className={cn('inline-flex items-center gap-1', className)} aria-hidden="true">
        {Array.from({ length: count }).map((_, idx) => (
          <span
            key={idx}
            className={cn(
              'loading-dot h-1.5 w-1.5 rounded-full motion-reduce:animate-none',
              dotClassName
            )}
            style={{
              animationDelay: `${idx * 0.08}s`,
            }}
          />
        ))}
      </span>
      <style jsx>{`
        .loading-dot {
          background: hsl(var(--foreground) / 0.5);
          animation: sweep 0.4s ease-in-out infinite;
        }
        @keyframes sweep {
          0%, 100% {
            opacity: 0.4;
          }
          50% {
            opacity: 1;
          }
        }
        @media (prefers-reduced-motion: reduce) {
          .loading-dot {
            animation: none;
          }
        }
      `}</style>
    </>
  );
}

export function ProgressBar({
  progress,
  total,
  label,
  className,
}: {
  progress: number;
  total: number;
  label?: string;
  className?: string;
}) {
  const percentage = Math.min(100, Math.round((progress / total) * 100));

  return (
    <div className={cn('space-y-2', className)}>
      {label && (
        <div className="flex items-center justify-between">
          <span>{label}</span>
          <span>
            {progress}/{total} ({percentage}%)
          </span>
        </div>
      )}
      <div className="w-full h-2 overflow-hidden">
        <div className="h-full" style={{ width: `${percentage}%` }} />
      </div>
    </div>
  );
}

export function StreamingIndicator({ active = true }: { active?: boolean }) {
  if (!active) return null;

  return (
    <div className="flex items-center gap-2">
      <LoadingDots />
      <span>Streaming...</span>
    </div>
  );
}

// Skeleton loaders for specific components
export function AgentOutputSkeleton() {
  return (
    <div className="space-y-4">
      <Skeleton className="h-16 w-full" />
      <Skeleton className="h-32 w-full" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-40 w-full" />
    </div>
  );
}

export function TaskListSkeleton() {
  return (
    <div className="space-y-3">
      {[1, 2, 3, 4].map((i) => (
        <div key={i} className="flex items-center gap-3 p-4">
          <Skeleton className="h-10 w-10" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
          <Skeleton className="h-6 w-20" />
        </div>
      ))}
    </div>
  );
}

export function SessionCardSkeleton() {
  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-6 w-16" />
      </div>
      <div className="space-y-2">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-2/3" />
      </div>
      <div className="flex gap-2">
        <Skeleton className="h-8 flex-1" />
        <Skeleton className="h-8 flex-1" />
      </div>
    </div>
  );
}
