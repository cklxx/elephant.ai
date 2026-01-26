'use client';

import { Component, ReactNode, ErrorInfo } from 'react';
import { AppError } from '@/lib/errors/AppError';
import { performanceMonitor } from '@/lib/analytics/performance';

interface SmartErrorBoundaryProps {
  children: ReactNode;
  fallback?: (error: AppError, reset: () => void, retryCount: number) => ReactNode;
  onError?: (error: AppError, errorInfo: ErrorInfo) => void;
  maxRetries?: number;
}

interface SmartErrorBoundaryState {
  hasError: boolean;
  error: AppError | null;
  errorInfo: ErrorInfo | null;
  retryCount: number;
}

/**
 * Enhanced ErrorBoundary with error classification and auto-retry
 * Integrates with AppError system and performance monitoring
 */
export class SmartErrorBoundary extends Component<
  SmartErrorBoundaryProps,
  SmartErrorBoundaryState
> {
  private resetTimeoutId: NodeJS.Timeout | null = null;

  constructor(props: SmartErrorBoundaryProps) {
    super(props);
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
      retryCount: 0,
    };
  }

  static getDerivedStateFromError(error: Error): Partial<SmartErrorBoundaryState> {
    const appError = error instanceof AppError ? error : AppError.from(error);
    return {
      hasError: true,
      error: appError,
    };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    const appError = error instanceof AppError ? error : AppError.from(error, {
      component: errorInfo.componentStack?.split('\n')[1]?.trim(),
    });

    // Track error for analytics
    performanceMonitor.track({
      name: 'error_boundary_catch',
      value: 1,
      unit: 'count',
      context: {
        errorType: appError.type,
        errorSeverity: appError.severity,
        recoverable: appError.recoverable,
        retryable: appError.retryable,
        retryCount: this.state.retryCount,
      },
    });

    // Log error details
    console.error('[SmartErrorBoundary] Caught error:', appError.toJSON());
    console.error('[SmartErrorBoundary] Error info:', errorInfo);

    // Update state
    this.setState({
      error: appError,
      errorInfo,
    });

    // Call custom error handler
    if (this.props.onError) {
      this.props.onError(appError, errorInfo);
    }

    // Auto-retry for retryable errors
    if (appError.retryable && this.state.retryCount < (this.props.maxRetries ?? 3)) {
      this.scheduleAutoRetry();
    }
  }

  scheduleAutoRetry = (): void => {
    const { retryCount } = this.state;
    const delay = Math.min(1000 * 2 ** retryCount, 10000); // Exponential backoff, max 10s

    console.log(`[SmartErrorBoundary] Scheduling auto-retry in ${delay}ms (attempt ${retryCount + 1})`);

    this.resetTimeoutId = setTimeout(() => {
      this.handleReset();
    }, delay);
  };

  handleReset = (): void => {
    if (this.resetTimeoutId) {
      clearTimeout(this.resetTimeoutId);
      this.resetTimeoutId = null;
    }

    this.setState((prevState) => ({
      hasError: false,
      error: null,
      errorInfo: null,
      retryCount: prevState.retryCount + 1,
    }));
  };

  componentWillUnmount(): void {
    if (this.resetTimeoutId) {
      clearTimeout(this.resetTimeoutId);
    }
  }

  render(): ReactNode {
    const { hasError, error, retryCount } = this.state;
    const { children, fallback } = this.props;

    if (hasError && error) {
      if (fallback) {
        return fallback(error, this.handleReset, retryCount);
      }

      // Minimal default fallback (should provide custom fallback)
      return (
        <div className="p-4 border border-destructive rounded-lg bg-destructive/10">
          <p className="font-semibold text-destructive mb-2">
            Error: {error.type}
          </p>
          <p className="text-sm text-muted-foreground mb-4">{error.message}</p>
          {error.retryable && retryCount < (this.props.maxRetries ?? 3) && (
            <p className="text-xs text-muted-foreground mb-2">
              Auto-retrying... (attempt {retryCount + 1})
            </p>
          )}
          <button
            onClick={this.handleReset}
            className="px-4 py-2 bg-destructive text-destructive-foreground rounded"
          >
            Retry Now
          </button>
        </div>
      );
    }

    return children;
  }
}
