import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSessionStore } from '../useSessionStore';

describe('useSessionStore', () => {
  beforeEach(() => {
    localStorage.clear();
    act(() => {
      useSessionStore.setState({
        currentSessionId: null,
        sessionHistory: [],
        sessionLabels: {},
      });
    });
    useSessionStore.persist?.clearStorage?.();
  });

  describe('Current Session Management', () => {
    it('initializes with null session', () => {
      const { result } = renderHook(() => useSessionStore());
      expect(result.current.currentSessionId).toBe(null);
    });

    it('sets the current session', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.setCurrentSession('session-123');
      });

      expect(result.current.currentSessionId).toBe('session-123');
    });

    it('clears the current session', () => {
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
    it('adds sessions to history', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.addToHistory('session-1');
      });

      expect(result.current.sessionHistory).toEqual(['session-1']);
    });

    it('prevents duplicate sessions in history', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.addToHistory('session-1');
        result.current.addToHistory('session-2');
        result.current.addToHistory('session-1');
      });

      expect(result.current.sessionHistory).toEqual(['session-2', 'session-1']);
    });

    it('limits history to the most recent 10 items', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        for (let i = 1; i <= 12; i++) {
          result.current.addToHistory(`session-${i}`);
        }
      });

      expect(result.current.sessionHistory).toHaveLength(10);
      expect(result.current.sessionHistory[0]).toBe('session-12');
      expect(result.current.sessionHistory).not.toContain('session-1');
    });

    it('keeps existing sessions in place when re-added', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.addToHistory('session-1');
        result.current.addToHistory('session-2');
        result.current.addToHistory('session-3');
      });

      expect(result.current.sessionHistory).toEqual([
        'session-3',
        'session-2',
        'session-1',
      ]);

      act(() => {
        result.current.addToHistory('session-1');
      });

      expect(result.current.sessionHistory).toEqual([
        'session-3',
        'session-2',
        'session-1',
      ]);
    });
  });

  describe('Persistence', () => {
    it('persists the store to localStorage', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.setCurrentSession('session-123');
        result.current.addToHistory('session-123');
        result.current.renameSession('session-123', 'Important workflow');
      });

      const stored = localStorage.getItem('alex-session-storage');
      expect(stored).toBeTruthy();

      const parsed = JSON.parse(stored!);
      expect(parsed.state.currentSessionId).toBe('session-123');
      expect(parsed.state.sessionHistory).toEqual(['session-123']);
      expect(parsed.state.sessionLabels['session-123']).toBe('Important workflow');
      expect(parsed.state.pinnedSessions).toBeUndefined();
    });

    it('restores persisted state on rehydrate', async () => {
      const mockState = {
        state: {
          currentSessionId: 'restored-session',
          sessionHistory: ['old-session'],
          sessionLabels: {
            'restored-session': 'Primary workflow',
          },
        },
        version: 0,
      };
      localStorage.setItem('alex-session-storage', JSON.stringify(mockState));

      await act(async () => {
        await useSessionStore.persist.rehydrate();
      });

      const { result } = renderHook(() => useSessionStore());

      expect(result.current.currentSessionId).toBe('restored-session');
      expect(result.current.sessionHistory).toEqual(['old-session']);
      expect(result.current.sessionLabels['restored-session']).toBe('Primary workflow');
    });

    it('drops legacy pinned metadata on persist', async () => {
      const legacyState = {
        state: {
          currentSessionId: 'legacy-session',
          sessionHistory: ['legacy-session'],
          pinnedSessions: ['legacy-session'],
        },
        version: 0,
      };
      localStorage.setItem('alex-session-storage', JSON.stringify(legacyState));

      await act(async () => {
        await useSessionStore.persist.rehydrate();
      });

      const { result } = renderHook(() => useSessionStore());
      expect((result.current as Record<string, unknown>).pinnedSessions).toBeUndefined();

      act(() => {
        result.current.addToHistory('legacy-session');
      });

      const stored = localStorage.getItem('alex-session-storage');
      expect(stored).toBeTruthy();
      const parsed = JSON.parse(stored!);
      expect(parsed.state.pinnedSessions).toBeUndefined();
      expect(parsed.state.sessionHistory).toEqual(['legacy-session']);
    });
  });

  describe('Edge Cases', () => {
    it('handles empty history', () => {
      const { result } = renderHook(() => useSessionStore());

      expect(result.current.sessionHistory).toEqual([]);
    });

    it('handles rapid session switches', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        for (let i = 1; i <= 5; i++) {
          result.current.setCurrentSession(`session-${i}`);
        }
      });

      expect(result.current.currentSessionId).toBe('session-5');
    });

    it('renames sessions and clears labels when blank', () => {
      const { result } = renderHook(() => useSessionStore());

      act(() => {
        result.current.renameSession('session-1', 'Focus project');
      });

      expect(result.current.sessionLabels['session-1']).toBe('Focus project');

      act(() => {
        result.current.renameSession('session-1', '   ');
      });

      expect(result.current.sessionLabels['session-1']).toBeUndefined();
    });
  });
});
