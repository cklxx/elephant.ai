import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSessionStore } from '../useSessionStore';

describe('useSessionStore', () => {
  beforeEach(() => {
    // Clear localStorage before each test
    localStorage.clear();
    // Reset store state
    const { result } = renderHook(() => useSessionStore());
    act(() => {
      result.current.clearCurrentSession();
    });
  });

  describe('Current Session Management', () => {
    it('should initialize with null session', () => {
      const { result } = renderHook(() => useSessionStore());
      expect(result.current.currentSessionId).toBe(null);
    });

    it('should set current session', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.setCurrentSession('session-123');
      });

      expect(result.current.currentSessionId).toBe('session-123');
    });

    it('should clear current session', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.setCurrentSession('session-123');
      });

      expect(result.current.currentSessionId).toBe('session-123');

      act(() => {
        result.current.clearCurrentSession();
      });

      expect(result.current.currentSessionId).toBe(null);
    });
  });

  describe('Session History', () => {
    it('should add session to history', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.addToHistory('session-1');
      });

      expect(result.current.sessionHistory).toContain('session-1');
    });

    it('should prevent duplicate sessions in history', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.addToHistory('session-1');
        result.current.addToHistory('session-2');
        result.current.addToHistory('session-1'); // Duplicate
      });

      // session-1 should be moved to front, not duplicated
      expect(result.current.sessionHistory).toEqual(['session-1', 'session-2']);
    });

    it('should limit history to 10 sessions', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        for (let i = 1; i <= 12; i++) {
          result.current.addToHistory(`session-${i}`);
        }
      });

      expect(result.current.sessionHistory).toHaveLength(10);
      expect(result.current.sessionHistory[0]).toBe('session-12'); // Most recent
      expect(result.current.sessionHistory).not.toContain('session-1'); // Oldest evicted
      expect(result.current.sessionHistory).not.toContain('session-2');
    });

    it('should move existing session to front when re-added', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.addToHistory('session-1');
        result.current.addToHistory('session-2');
        result.current.addToHistory('session-3');
      });

      expect(result.current.sessionHistory).toEqual(['session-3', 'session-2', 'session-1']);

      act(() => {
        result.current.addToHistory('session-1'); // Re-add old session
      });

      expect(result.current.sessionHistory).toEqual(['session-1', 'session-3', 'session-2']);
    });
  });

  describe('Persistence', () => {
    it('should persist to localStorage', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.setCurrentSession('session-123');
        result.current.addToHistory('session-123');
      });

      // Check localStorage directly
      const stored = localStorage.getItem('alex-session-storage');
      expect(stored).toBeTruthy();

      const parsed = JSON.parse(stored!);
      expect(parsed.state.currentSessionId).toBe('session-123');
      expect(parsed.state.sessionHistory).toContain('session-123');
    });

    it('should restore from localStorage on mount', () => {
      // Pre-populate localStorage
      const mockState = {
        state: {
          currentSessionId: 'restored-session',
          sessionHistory: ['restored-session', 'old-session'],
        },
        version: 0,
      };
      localStorage.setItem('alex-session-storage', JSON.stringify(mockState));

      // Mount hook
      const { result } = renderHook(() => useSessionStore());

      expect(result.current.currentSessionId).toBe('restored-session');
      expect(result.current.sessionHistory).toEqual(['restored-session', 'old-session']);
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty session history', () => {
      const { result } = renderHook(() => useSessionStore());

      expect(result.current.sessionHistory).toEqual([]);
    });

    it('should handle rapid session switches', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        for (let i = 1; i <= 5; i++) {
          result.current.setCurrentSession(`session-${i}`);
        }
      });

      expect(result.current.currentSessionId).toBe('session-5');
    });
  });
});
