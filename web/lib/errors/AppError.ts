/**
 * Application Error Classification System
 * Provides structured error handling with retry and recovery capabilities
 */

export type ErrorType = 'network' | 'auth' | 'validation' | 'fatal' | 'unknown';
export type ErrorSeverity = 'low' | 'medium' | 'high' | 'critical';

export interface ErrorContext {
  userId?: string;
  sessionId?: string;
  url?: string;
  component?: string;
  action?: string;
  [key: string]: unknown;
}

export class AppError extends Error {
  public readonly type: ErrorType;
  public readonly severity: ErrorSeverity;
  public readonly recoverable: boolean;
  public readonly retryable: boolean;
  public readonly context: ErrorContext;
  public readonly timestamp: number;
  public readonly originalError?: Error;

  constructor(
    message: string,
    options: {
      type?: ErrorType;
      severity?: ErrorSeverity;
      recoverable?: boolean;
      retryable?: boolean;
      context?: ErrorContext;
      originalError?: Error;
    } = {}
  ) {
    super(message);
    this.name = 'AppError';
    this.type = options.type ?? 'unknown';
    this.severity = options.severity ?? 'medium';
    this.recoverable = options.recoverable ?? false;
    this.retryable = options.retryable ?? false;
    this.context = options.context ?? {};
    this.timestamp = Date.now();
    this.originalError = options.originalError;

    // Maintain proper stack trace
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, AppError);
    }
  }

  /**
   * Create a network error (e.g., fetch failed)
   */
  static network(message: string, context?: ErrorContext): AppError {
    return new AppError(message, {
      type: 'network',
      severity: 'medium',
      recoverable: true,
      retryable: true,
      context,
    });
  }

  /**
   * Create an authentication error
   */
  static auth(message: string, context?: ErrorContext): AppError {
    return new AppError(message, {
      type: 'auth',
      severity: 'high',
      recoverable: true,
      retryable: false,
      context,
    });
  }

  /**
   * Create a validation error
   */
  static validation(message: string, context?: ErrorContext): AppError {
    return new AppError(message, {
      type: 'validation',
      severity: 'low',
      recoverable: true,
      retryable: false,
      context,
    });
  }

  /**
   * Create a fatal error
   */
  static fatal(message: string, context?: ErrorContext): AppError {
    return new AppError(message, {
      type: 'fatal',
      severity: 'critical',
      recoverable: false,
      retryable: false,
      context,
    });
  }

  /**
   * Classify an unknown error
   */
  static from(error: unknown, context?: ErrorContext): AppError {
    if (error instanceof AppError) {
      return error;
    }

    if (error instanceof Error) {
      // Try to classify based on error message
      const message = error.message.toLowerCase();

      if (message.includes('fetch') || message.includes('network')) {
        return AppError.network(error.message, {
          ...context,
          originalError: error,
        });
      }

      if (message.includes('auth') || message.includes('unauthorized')) {
        return AppError.auth(error.message, {
          ...context,
          originalError: error,
        });
      }

      if (message.includes('invalid') || message.includes('validation')) {
        return AppError.validation(error.message, {
          ...context,
          originalError: error,
        });
      }

      return new AppError(error.message, {
        type: 'unknown',
        severity: 'medium',
        recoverable: false,
        retryable: false,
        context,
        originalError: error,
      });
    }

    return new AppError(String(error), {
      type: 'unknown',
      severity: 'medium',
      recoverable: false,
      retryable: false,
      context,
    });
  }

  /**
   * Get error details for logging
   */
  toJSON() {
    return {
      name: this.name,
      message: this.message,
      type: this.type,
      severity: this.severity,
      recoverable: this.recoverable,
      retryable: this.retryable,
      context: this.context,
      timestamp: this.timestamp,
      stack: this.stack,
      originalError: this.originalError
        ? {
            name: this.originalError.name,
            message: this.originalError.message,
            stack: this.originalError.stack,
          }
        : undefined,
    };
  }
}
