import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SmartErrorBoundary } from '../SmartErrorBoundary';
import { AppError } from '@/lib/errors/AppError';
import { performanceMonitor } from '@/lib/analytics/performance';

// Suppress React error boundary console noise
const originalConsoleError = console.error;
beforeEach(() => {
  console.error = vi.fn();
});
afterEach(() => {
  console.error = originalConsoleError;
  vi.useRealTimers();
  vi.restoreAllMocks();
});

function ThrowingComponent({ error }: { error?: Error }) {
  if (error) {
    throw error;
  }
  return <div data-testid="child-content">Working</div>;
}

describe('SmartErrorBoundary', () => {
  it('renders children when no error', () => {
    render(
      <SmartErrorBoundary level="page">
        <div data-testid="child">OK</div>
      </SmartErrorBoundary>,
    );
    expect(screen.getByTestId('child')).toBeInTheDocument();
  });

  it('catches error and classifies via AppError.from', () => {
    const networkError = new Error('Network fetch failed');
    render(
      <SmartErrorBoundary level="section">
        <ThrowingComponent error={networkError} />
      </SmartErrorBoundary>,
    );
    expect(screen.getByTestId('section-error-boundary')).toBeInTheDocument();
  });

  it('shows retry UI for retryable errors', () => {
    const networkError = new Error('Network fetch failed');
    render(
      <SmartErrorBoundary level="section">
        <ThrowingComponent error={networkError} />
      </SmartErrorBoundary>,
    );
    expect(screen.getByTestId('error-retryable')).toBeInTheDocument();
    expect(screen.getByTestId('retry-button')).toBeInTheDocument();
  });

  it('renders page-level layout for level=page', () => {
    const fatalError = new Error('Something crashed badly');
    render(
      <SmartErrorBoundary level="page">
        <ThrowingComponent error={fatalError} />
      </SmartErrorBoundary>,
    );
    expect(screen.getByTestId('page-error-boundary')).toBeInTheDocument();
  });

  it('renders section-level layout for level=section', () => {
    const fatalError = new Error('Something crashed badly');
    render(
      <SmartErrorBoundary level="section">
        <ThrowingComponent error={fatalError} />
      </SmartErrorBoundary>,
    );
    expect(screen.getByTestId('section-error-boundary')).toBeInTheDocument();
  });

  it('shows auth error with re-authenticate link', () => {
    const authError = new Error('Unauthorized access');
    render(
      <SmartErrorBoundary level="section">
        <ThrowingComponent error={authError} />
      </SmartErrorBoundary>,
    );
    expect(screen.getByTestId('error-auth')).toBeInTheDocument();
    expect(screen.getByTestId('auth-login-link')).toHaveAttribute('href', '/login');
  });

  it('retries on retry button click', async () => {
    const user = userEvent.setup();
    let shouldThrow = true;
    function MaybeThrow() {
      if (shouldThrow) {
        throw new Error('Network fetch failed');
      }
      return <div data-testid="recovered">Recovered</div>;
    }

    render(
      <SmartErrorBoundary level="section" maxRetries={3}>
        <MaybeThrow />
      </SmartErrorBoundary>,
    );

    expect(screen.getByTestId('error-retryable')).toBeInTheDocument();

    shouldThrow = false;
    await user.click(screen.getByTestId('retry-button'));

    expect(screen.getByTestId('recovered')).toBeInTheDocument();
  });

  it('shows fatal error after retries exhausted', () => {
    // Force a non-retryable (unknown) error to render fatal immediately
    const fatalError = new Error('Something totally broke');
    render(
      <SmartErrorBoundary level="section" maxRetries={0}>
        <ThrowingComponent error={fatalError} />
      </SmartErrorBoundary>,
    );
    expect(screen.getByTestId('error-fatal')).toBeInTheDocument();
  });

  it('exhausts retries and falls back to fatal error', async () => {
    const user = userEvent.setup();

    // Always throws a retryable error
    function AlwaysNetworkError() {
      throw new Error('Network fetch failed');
    }

    render(
      <SmartErrorBoundary level="section" maxRetries={2}>
        <AlwaysNetworkError />
      </SmartErrorBoundary>,
    );

    // First catch: retryable
    expect(screen.getByTestId('error-retryable')).toBeInTheDocument();
    expect(screen.getByText(/0\/2/)).toBeInTheDocument();

    // First manual retry
    await user.click(screen.getByTestId('retry-button'));
    expect(screen.getByTestId('error-retryable')).toBeInTheDocument();
    expect(screen.getByText(/1\/2/)).toBeInTheDocument();

    // Second manual retry — exhausted, should show fatal
    await user.click(screen.getByTestId('retry-button'));
    expect(screen.getByTestId('error-fatal')).toBeInTheDocument();
  });

  it('tracks error via performanceMonitor', () => {
    const trackSpy = vi.spyOn(performanceMonitor, 'track');
    const error = new Error('Something went wrong');

    render(
      <SmartErrorBoundary level="page">
        <ThrowingComponent error={error} />
      </SmartErrorBoundary>,
    );

    expect(trackSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'error_boundary_catch',
        value: 1,
        unit: 'count',
        context: expect.objectContaining({
          level: 'page',
          errorType: expect.any(String),
          severity: expect.any(String),
        }),
      }),
    );
  });

  it('renders custom fallback when provided', () => {
    const customFallback = (error: AppError, reset: () => void) => (
      <div data-testid="custom-fallback">
        Custom: {error.message}
        <button onClick={reset}>Reset</button>
      </div>
    );

    render(
      <SmartErrorBoundary level="page" fallback={customFallback}>
        <ThrowingComponent error={new Error('test')} />
      </SmartErrorBoundary>,
    );

    expect(screen.getByTestId('custom-fallback')).toBeInTheDocument();
    expect(screen.getByText(/Custom: test/)).toBeInTheDocument();
  });

  it('calls onError callback when error is caught', () => {
    const onError = vi.fn();
    const error = new Error('test error');

    render(
      <SmartErrorBoundary level="section" onError={onError}>
        <ThrowingComponent error={error} />
      </SmartErrorBoundary>,
    );

    expect(onError).toHaveBeenCalledWith(
      expect.any(AppError),
      expect.objectContaining({
        componentStack: expect.any(String),
      }),
    );
  });

  it('resets error state via reset button on fatal error', async () => {
    const user = userEvent.setup();
    let shouldThrow = true;
    function MaybeThrow() {
      if (shouldThrow) {
        throw new Error('Fatal crash');
      }
      return <div data-testid="recovered">Back</div>;
    }

    render(
      <SmartErrorBoundary level="section" maxRetries={0}>
        <MaybeThrow />
      </SmartErrorBoundary>,
    );

    expect(screen.getByTestId('error-fatal')).toBeInTheDocument();

    shouldThrow = false;
    await user.click(screen.getByTestId('reset-button'));

    expect(screen.getByTestId('recovered')).toBeInTheDocument();
  });
});
