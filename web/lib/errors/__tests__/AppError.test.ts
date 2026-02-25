import { describe, it, expect } from 'vitest';
import { AppError } from '../AppError';

describe('AppError', () => {
  describe('constructor', () => {
    it('creates error with default values', () => {
      const error = new AppError('Test error');
      expect(error.message).toBe('Test error');
      expect(error.type).toBe('unknown');
      expect(error.severity).toBe('medium');
      expect(error.recoverable).toBe(false);
      expect(error.retryable).toBe(false);
    });

    it('creates error with custom values', () => {
      const error = new AppError('Network error', {
        type: 'network',
        severity: 'high',
        recoverable: true,
        retryable: true,
        context: { userId: '123' },
      });

      expect(error.type).toBe('network');
      expect(error.severity).toBe('high');
      expect(error.recoverable).toBe(true);
      expect(error.retryable).toBe(true);
      expect(error.context.userId).toBe('123');
    });
  });

  describe('static factory methods', () => {
    it('creates network error', () => {
      const error = AppError.network('Connection failed');
      expect(error.type).toBe('network');
      expect(error.recoverable).toBe(true);
      expect(error.retryable).toBe(true);
    });

    it('creates auth error', () => {
      const error = AppError.auth('Unauthorized');
      expect(error.type).toBe('auth');
      expect(error.recoverable).toBe(true);
      expect(error.retryable).toBe(false);
    });

    it('creates validation error', () => {
      const error = AppError.validation('Invalid input');
      expect(error.type).toBe('validation');
      expect(error.severity).toBe('low');
    });

    it('creates fatal error', () => {
      const error = AppError.fatal('Critical failure');
      expect(error.type).toBe('fatal');
      expect(error.severity).toBe('critical');
      expect(error.recoverable).toBe(false);
    });
  });

  describe('from', () => {
    it('returns AppError as-is', () => {
      const original = AppError.network('Test');
      const result = AppError.from(original);
      expect(result).toBe(original);
    });

    it('classifies Error by message - network', () => {
      const error = new Error('Network fetch failed');
      const result = AppError.from(error);
      expect(result.type).toBe('network');
      expect(result.retryable).toBe(true);
    });

    it('classifies Error by message - auth', () => {
      const error = new Error('Unauthorized access');
      const result = AppError.from(error);
      expect(result.type).toBe('auth');
    });

    it('classifies Error by message - validation', () => {
      const error = new Error('Invalid email format');
      const result = AppError.from(error);
      expect(result.type).toBe('validation');
    });

    it('handles unknown Error', () => {
      const error = new Error('Something weird happened');
      const result = AppError.from(error);
      expect(result.type).toBe('unknown');
      expect(result.message).toBe('Something weird happened');
    });

    it('handles non-Error values', () => {
      const result = AppError.from('String error');
      expect(result.message).toBe('String error');
      expect(result.type).toBe('unknown');
    });
  });

  describe('toJSON', () => {
    it('serializes error details', () => {
      const error = AppError.network('Test', { sessionId: '123' });
      const json = error.toJSON();

      expect(json.name).toBe('AppError');
      expect(json.message).toBe('Test');
      expect(json.type).toBe('network');
      expect(json.severity).toBe('medium');
      expect(json.context.sessionId).toBe('123');
      expect(json.timestamp).toBeGreaterThan(0);
    });

    it('includes original error details', () => {
      const original = new Error('Original error');
      const error = new AppError('Wrapped', { originalError: original });
      const json = error.toJSON();

      expect(json.originalError).toBeDefined();
      expect(json.originalError?.message).toBe('Original error');
    });
  });
});
