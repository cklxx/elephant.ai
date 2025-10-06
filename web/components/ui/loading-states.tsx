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
      <Loader2 className={cn('animate-spin text-blue-600', sizeClasses[size])} />
      {label && <p className="text-sm text-gray-600 font-medium">{label}</p>}
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
    <div className={cn('flex items-center gap-3 p-4 rounded-lg', className)}>
      {status === 'loading' && (
        <>
          <Loader2 className="h-5 w-5 animate-spin text-blue-600" />
          <span className="text-sm text-gray-700">{loadingText}</span>
        </>
      )}
      {status === 'success' && (
        <>
          <CheckCircle2 className="h-5 w-5 text-green-600" />
          <span className="text-sm text-green-700 font-medium">{successText}</span>
        </>
      )}
      {status === 'error' && (
        <>
          <AlertCircle className="h-5 w-5 text-red-600" />
          <span className="text-sm text-red-700 font-medium">{errorText}</span>
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
        'flex items-center justify-center bg-white/80 backdrop-blur-sm z-50',
        fullscreen ? 'fixed inset-0' : 'absolute inset-0'
      )}
    >
      <div className="glass-card p-8 rounded-2xl shadow-strong">
        <LoadingSpinner size="lg" label={message} />
      </div>
    </div>
  );
}

export function PulsingDot({ color = 'blue' }: { color?: 'blue' | 'green' | 'red' | 'yellow' }) {
  const colorClasses = {
    blue: 'bg-blue-500',
    green: 'bg-green-500',
    red: 'bg-red-500',
    yellow: 'bg-yellow-500',
  };

  return (
    <span className="relative flex h-3 w-3">
      <span
        className={cn(
          'animate-ping absolute inline-flex h-full w-full rounded-full opacity-75',
          colorClasses[color]
        )}
      ></span>
      <span className={cn('relative inline-flex rounded-full h-3 w-3', colorClasses[color])}></span>
    </span>
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
        <div className="flex items-center justify-between text-sm">
          <span className="text-gray-700 font-medium">{label}</span>
          <span className="text-gray-500">
            {progress}/{total} ({percentage}%)
          </span>
        </div>
      )}
      <div className="w-full bg-gray-200 rounded-full h-2 overflow-hidden">
        <div
          className="bg-gradient-to-r from-blue-500 to-blue-600 h-full rounded-full transition-all duration-500 ease-out"
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  );
}

export function StreamingIndicator({ active = true }: { active?: boolean }) {
  if (!active) return null;

  return (
    <div className="flex items-center gap-2 text-sm text-gray-600">
      <div className="flex gap-1">
        <span className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '0ms' }}></span>
        <span className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '150ms' }}></span>
        <span className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '300ms' }}></span>
      </div>
      <span className="font-medium">Streaming...</span>
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
        <div key={i} className="flex items-center gap-3 p-4 border border-gray-200 rounded-lg">
          <Skeleton className="h-10 w-10 rounded-full" />
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
    <div className="glass-card p-6 rounded-xl shadow-soft space-y-4">
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
