'use client';

import React, { Component, ReactNode, ErrorInfo } from 'react';
import { AlertTriangle, RefreshCw, Home } from 'lucide-react';
import Link from 'next/link';

interface ErrorBoundaryProps {
  children: ReactNode;
  fallback?: (error: Error, reset: () => void) => ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
}

/**
 * ErrorBoundary component for catching and handling React errors
 * Follows React 18 error boundary best practices
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
    };
  }

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    // Update state so the next render will show the fallback UI
    return {
      hasError: true,
      error,
    };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    // Log error details for debugging
    console.error('[ErrorBoundary] Caught error:', error);
    console.error('[ErrorBoundary] Error info:', errorInfo);

    // Update state with error details
    this.setState({
      error,
      errorInfo,
    });

    // You can also log the error to an error reporting service here
    // Example: logErrorToService(error, errorInfo);
  }

  handleReset = (): void => {
    this.setState({
      hasError: false,
      error: null,
      errorInfo: null,
    });
  };

  render(): ReactNode {
    const { hasError, error, errorInfo } = this.state;
    const { children, fallback } = this.props;

    if (hasError && error) {
      // If a custom fallback is provided, use it
      if (fallback) {
        return fallback(error, this.handleReset);
      }

      // Default fallback UI
      return (
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-destructive/10 via-amber-50 to-primary/10 p-4">
          <div className="max-w-2xl w-full">
            <div className="bg-white rounded-xl p-8 border border-destructive/30">
              {/* Header */}
              <div className="flex items-center gap-4 mb-6">
                <div className="p-3 bg-destructive/10 rounded-full">
                  <AlertTriangle className="h-8 w-8 text-destructive" />
                </div>
                <div>
                  <h1 className="text-2xl font-bold text-gray-900">
                    Something went wrong
                  </h1>
                  <p className="text-sm text-gray-600 mt-1">
                    An unexpected error occurred in the application
                  </p>
                </div>
              </div>

              {/* Error details */}
              <div className="mb-6">
                <div className="bg-destructive/10 border border-destructive/30 rounded-lg p-4">
                  <p className="text-sm font-semibold text-destructive mb-2">
                    Error Message:
                  </p>
                  <p className="text-sm text-destructive font-mono">
                    {error.message || 'Unknown error'}
                  </p>
                </div>

                {/* Stack trace (only in development) */}
                {process.env.NODE_ENV === 'development' && errorInfo && (
                  <details className="mt-4">
                    <summary className="text-sm font-semibold text-gray-700 cursor-pointer hover:text-gray-900 mb-2">
                      Show error details (development only)
                    </summary>
                    <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 overflow-auto max-h-96">
                      <pre className="text-xs text-gray-800 font-mono whitespace-pre-wrap">
                        {error.stack}
                        {'\n\n'}
                        Component Stack:
                        {errorInfo.componentStack}
                      </pre>
                    </div>
                  </details>
                )}
              </div>

              {/* Action buttons */}
              <div className="flex gap-3">
                <button
                  onClick={this.handleReset}
                  className="flex items-center gap-2 px-4 py-2 bg-destructive text-destructive-foreground rounded-lg hover:bg-destructive/90 transition-colors font-medium"
                >
                  <RefreshCw className="h-4 w-4" />
                  Try Again
                </button>
                <Link
                  href="/"
                  className="flex items-center gap-2 px-4 py-2 rounded-lg border border-primary/30 bg-background text-foreground hover:bg-primary/10 transition-colors font-medium"
                >
                  <Home className="h-4 w-4" />
                  Go Home
                </Link>
              </div>

              {/* Help text */}
              <div className="mt-6 pt-6 border-t border-gray-200">
                <p className="text-xs text-gray-600">
                  If this problem persists, please check the browser console for more details
                  or contact support.
                </p>
              </div>
            </div>
          </div>
        </div>
      );
    }

    return children;
  }
}

/**
 * Hook-based error boundary wrapper for functional components
 * Note: This doesn't replace ErrorBoundary class, but provides a convenient way to add error handling
 */
export function withErrorBoundary<P extends object>(
  Component: React.ComponentType<P>,
  fallback?: (error: Error, reset: () => void) => ReactNode
): React.FC<P> {
  const WrappedComponent: React.FC<P> = (props) => (
    <ErrorBoundary fallback={fallback}>
      <Component {...props} />
    </ErrorBoundary>
  );

  WrappedComponent.displayName = `withErrorBoundary(${Component.displayName || Component.name || 'Component'})`;

  return WrappedComponent;
}
