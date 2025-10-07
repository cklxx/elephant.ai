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
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-red-50 via-orange-50 to-yellow-50 p-4">
          <div className="max-w-2xl w-full">
            <div className="bg-white rounded-xl shadow-xl p-8 border border-red-200">
              {/* Header */}
              <div className="flex items-center gap-4 mb-6">
                <div className="p-3 bg-red-100 rounded-full">
                  <AlertTriangle className="h-8 w-8 text-red-600" />
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
                <div className="bg-red-50 border border-red-200 rounded-lg p-4">
                  <p className="text-sm font-semibold text-red-900 mb-2">
                    Error Message:
                  </p>
                  <p className="text-sm text-red-800 font-mono">
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
                  className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors font-medium"
                >
                  <RefreshCw className="h-4 w-4" />
                  Try Again
                </button>
                <Link
                  href="/"
                  className="flex items-center gap-2 px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 transition-colors font-medium"
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
