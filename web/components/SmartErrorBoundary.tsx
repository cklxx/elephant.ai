'use client';

import { Component, ReactNode, ErrorInfo } from 'react';
import { AlertTriangle, RefreshCw, Home, LogIn } from 'lucide-react';
import Link from 'next/link';
import { AppError } from '@/lib/errors/AppError';
import { performanceMonitor } from '@/lib/analytics/performance';

interface SmartErrorBoundaryProps {
  children: ReactNode;
  level: 'page' | 'section';
  maxRetries?: number;
  fallback?: (error: AppError, reset: () => void) => ReactNode;
  onError?: (error: AppError, errorInfo: ErrorInfo) => void;
}

interface SmartErrorBoundaryState {
  hasError: boolean;
  appError: AppError | null;
  errorInfo: ErrorInfo | null;
  retryCount: number;
}

export class SmartErrorBoundary extends Component<SmartErrorBoundaryProps, SmartErrorBoundaryState> {
  private retryTimeoutId: ReturnType<typeof setTimeout> | null = null;

  static defaultProps = {
    maxRetries: 3,
  };

  constructor(props: SmartErrorBoundaryProps) {
    super(props);
    this.state = {
      hasError: false,
      appError: null,
      errorInfo: null,
      retryCount: 0,
    };
  }

  static getDerivedStateFromError(error: Error): Partial<SmartErrorBoundaryState> {
    return {
      hasError: true,
      appError: AppError.from(error),
    };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    const appError = AppError.from(error);

    this.setState({ appError, errorInfo });

    performanceMonitor.track({
      name: 'error_boundary_catch',
      value: 1,
      unit: 'count',
      context: {
        level: this.props.level,
        errorType: appError.type,
        severity: appError.severity,
        retryable: appError.retryable,
        component: errorInfo.componentStack?.slice(0, 200),
      },
    });

    this.props.onError?.(appError, errorInfo);
  }

  componentWillUnmount(): void {
    if (this.retryTimeoutId) {
      clearTimeout(this.retryTimeoutId);
    }
  }

  handleReset = (): void => {
    if (this.retryTimeoutId) {
      clearTimeout(this.retryTimeoutId);
      this.retryTimeoutId = null;
    }
    this.setState({
      hasError: false,
      appError: null,
      errorInfo: null,
      retryCount: 0,
    });
  };

  handleRetry = (): void => {
    const { maxRetries = 3 } = this.props;
    const nextRetryCount = this.state.retryCount + 1;

    if (nextRetryCount > maxRetries) {
      return;
    }

    this.setState({
      hasError: false,
      appError: null,
      errorInfo: null,
      retryCount: nextRetryCount,
    });
  };

  scheduleAutoRetry = (): void => {
    const { maxRetries = 3 } = this.props;
    const { retryCount, appError } = this.state;

    if (!appError?.retryable || retryCount >= maxRetries) {
      return;
    }

    const delay = Math.min(1000 * 2 ** retryCount, 10000);
    this.retryTimeoutId = setTimeout(() => {
      this.retryTimeoutId = null;
      this.handleRetry();
    }, delay);
  };

  componentDidUpdate(_prevProps: SmartErrorBoundaryProps, prevState: SmartErrorBoundaryState): void {
    if (this.state.hasError && !prevState.hasError && this.state.appError?.retryable) {
      this.scheduleAutoRetry();
    }
  }

  renderAuthError(appError: AppError): ReactNode {
    return (
      <div className="flex flex-col items-center gap-4 p-6 text-center" data-testid="error-auth">
        <div className="p-3 bg-amber-100 rounded-full">
          <LogIn className="h-6 w-6 text-amber-700" />
        </div>
        <div>
          <h3 className="text-lg font-semibold text-foreground">Session expired</h3>
          <p className="mt-1 text-sm text-muted-foreground">{appError.message}</p>
        </div>
        <Link
          href="/login"
          className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
          data-testid="auth-login-link"
        >
          <LogIn className="h-4 w-4" />
          Re-authenticate
        </Link>
      </div>
    );
  }

  renderRetryableError(appError: AppError): ReactNode {
    const { maxRetries = 3 } = this.props;
    const { retryCount } = this.state;
    const retriesExhausted = retryCount >= maxRetries;

    if (retriesExhausted) {
      return this.renderFatalError(appError);
    }

    return (
      <div className="flex flex-col items-center gap-3 p-4 text-center" data-testid="error-retryable">
        <AlertTriangle className="h-5 w-5 text-amber-500" />
        <p className="text-sm text-foreground">{appError.message}</p>
        <p className="text-xs text-muted-foreground">
          Retry {retryCount}/{maxRetries} — retrying automatically...
        </p>
        <button
          onClick={this.handleRetry}
          className="inline-flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-sm font-medium text-foreground hover:bg-muted transition-colors"
          data-testid="retry-button"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          Retry now
        </button>
      </div>
    );
  }

  renderFatalError(appError: AppError): ReactNode {
    const { errorInfo } = this.state;

    const errorContent = (
      <>
        <div className="flex items-center gap-3">
          <div className="p-2.5 bg-destructive/10 rounded-full">
            <AlertTriangle className="h-6 w-6 text-destructive" />
          </div>
          <div>
            <h3 className="text-lg font-semibold text-foreground">Something went wrong</h3>
            <p className="mt-0.5 text-sm text-muted-foreground">
              An unexpected error occurred
            </p>
          </div>
        </div>

        <div className="bg-destructive/10 border border-destructive/30 rounded-lg p-3">
          <p className="text-sm text-destructive font-mono">{appError.message}</p>
        </div>

        {process.env.NODE_ENV === 'development' && errorInfo && (
          <details className="text-left" data-testid="error-stack-details">
            <summary className="text-xs font-semibold text-muted-foreground cursor-pointer hover:text-foreground">
              Stack trace (dev only)
            </summary>
            <pre className="mt-2 overflow-auto max-h-48 rounded-lg bg-muted p-3 text-xs font-mono text-muted-foreground whitespace-pre-wrap">
              {appError.stack}
              {'\n\n'}
              Component Stack:
              {errorInfo.componentStack}
            </pre>
          </details>
        )}

        <div className="flex gap-2">
          <button
            onClick={this.handleReset}
            className="inline-flex items-center gap-2 rounded-lg bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 transition-colors"
            data-testid="reset-button"
          >
            <RefreshCw className="h-4 w-4" />
            Try Again
          </button>
          <Link
            href="/"
            className="inline-flex items-center gap-2 rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-muted transition-colors"
          >
            <Home className="h-4 w-4" />
            Go Home
          </Link>
        </div>
      </>
    );

    return <div className="flex flex-col gap-4" data-testid="error-fatal">{errorContent}</div>;
  }

  renderSectionError(appError: AppError): ReactNode {
    if (appError.type === 'auth') {
      return (
        <div className="rounded-xl border border-amber-200 bg-amber-50/50 p-4" data-testid="section-error-boundary">
          {this.renderAuthError(appError)}
        </div>
      );
    }

    if (appError.retryable) {
      return (
        <div className="rounded-xl border border-border bg-card p-4" data-testid="section-error-boundary">
          {this.renderRetryableError(appError)}
        </div>
      );
    }

    return (
      <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-4" data-testid="section-error-boundary">
        {this.renderFatalError(appError)}
      </div>
    );
  }

  renderPageError(appError: AppError): ReactNode {
    if (appError.type === 'auth') {
      return (
        <div className="min-h-screen flex items-center justify-center p-4" data-testid="page-error-boundary">
          <div className="max-w-md w-full rounded-xl border border-amber-200 bg-white p-8">
            {this.renderAuthError(appError)}
          </div>
        </div>
      );
    }

    if (appError.retryable) {
      const { maxRetries = 3 } = this.props;
      const { retryCount } = this.state;
      if (retryCount < maxRetries) {
        return (
          <div className="min-h-screen flex items-center justify-center p-4" data-testid="page-error-boundary">
            <div className="max-w-md w-full rounded-xl border border-border bg-white p-8">
              {this.renderRetryableError(appError)}
            </div>
          </div>
        );
      }
    }

    return (
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-destructive/10 via-amber-50 to-primary/10 p-4" data-testid="page-error-boundary">
        <div className="max-w-2xl w-full rounded-xl bg-white border border-destructive/30 p-8">
          {this.renderFatalError(appError)}
        </div>
      </div>
    );
  }

  render(): ReactNode {
    const { hasError, appError } = this.state;
    const { children, level, fallback } = this.props;

    if (hasError && appError) {
      if (fallback) {
        return fallback(appError, this.handleReset);
      }

      return level === 'section'
        ? this.renderSectionError(appError)
        : this.renderPageError(appError);
    }

    return children;
  }
}
