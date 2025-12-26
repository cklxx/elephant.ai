// Zustand store for session management

import { create } from 'zustand';
import { persist } from 'zustand/middleware';

const MAX_HISTORY = 10;

interface SessionState {
  currentSessionId: string | null;
  sessionHistory: string[];
  sessionLabels: Record<string, string>;
  setCurrentSession: (sessionId: string) => void;
  clearCurrentSession: () => void;
  addToHistory: (sessionId: string) => void;
  removeSession: (sessionId: string) => void;
  renameSession: (sessionId: string, label: string) => void;
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      currentSessionId: null,
      sessionHistory: [],
      sessionLabels: {},

      setCurrentSession: (sessionId: string) =>
        set({ currentSessionId: sessionId }),

      clearCurrentSession: () =>
        set({ currentSessionId: null }),

      addToHistory: (sessionId: string) =>
        set((state) => {
          const currentHistory = state.sessionHistory ?? [];
          if (!sessionId || currentHistory.includes(sessionId)) {
            return state;
          }
          const nextHistory = [sessionId, ...currentHistory];

          return {
            sessionHistory: nextHistory.slice(0, MAX_HISTORY),
          };
        }),

      removeSession: (sessionId: string) =>
        set((state) => {
          const nextState: Partial<SessionState> = {};

          if (state.sessionHistory.includes(sessionId)) {
            nextState.sessionHistory = state.sessionHistory.filter((id) => id !== sessionId);
          }

          if (state.sessionLabels[sessionId]) {
            const nextLabels = { ...state.sessionLabels };
            delete nextLabels[sessionId];
            nextState.sessionLabels = nextLabels;
          }

          if (state.currentSessionId === sessionId) {
            nextState.currentSessionId = null;
          }

          return nextState;
        }),

      renameSession: (sessionId: string, label: string) =>
        set((state) => {
          const trimmed = label.trim();
          const nextLabels = { ...(state.sessionLabels ?? {}) };

          if (!trimmed) {
            delete nextLabels[sessionId];
          } else {
            nextLabels[sessionId] = trimmed;
          }

          return { sessionLabels: nextLabels };
        }),
    }),
    {
      name: 'alex-session-storage',
      merge: (persistedState, currentState) => {
        const persisted = (persistedState as Partial<SessionState>) ?? {};

        return {
          ...currentState,
          currentSessionId: persisted.currentSessionId ?? null,
          sessionHistory: persisted.sessionHistory ?? [],
          sessionLabels: persisted.sessionLabels ?? {},
        };
      },
      partialize: (state) => ({
        currentSessionId: state.currentSessionId,
        sessionHistory: state.sessionHistory,
        sessionLabels: state.sessionLabels,
      }),
    }
  )
);

// React Query hooks for sessions
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';

export function useSessions() {
  return useQuery({
    queryKey: ['sessions'],
    queryFn: () => apiClient.listSessions(),
  });
}

export function useSessionDetails(sessionId: string | null) {
  return useQuery({
    queryKey: ['session', sessionId],
    queryFn: () => {
      if (!sessionId) throw new Error('Session ID is required');
      return apiClient.getSessionDetails(sessionId);
    },
    enabled: !!sessionId,
  });
}

export function useDeleteSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (sessionId: string) => apiClient.deleteSession(sessionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
    },
  });
}

export function useForkSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (sessionId: string) => apiClient.forkSession(sessionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
    },
  });
}
