// Zustand store for session management

import { create } from 'zustand';
import { persist } from 'zustand/middleware';

const MAX_HISTORY = 10;
const MAX_PINNED = 5;

interface SessionState {
  currentSessionId: string | null;
  sessionHistory: string[];
  pinnedSessions: string[];
  sessionLabels: Record<string, string>;
  setCurrentSession: (sessionId: string) => void;
  clearCurrentSession: () => void;
  addToHistory: (sessionId: string) => void;
  renameSession: (sessionId: string, label: string) => void;
  togglePinSession: (sessionId: string) => void;
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      currentSessionId: null,
      sessionHistory: [],
      pinnedSessions: [],
      sessionLabels: {},

      setCurrentSession: (sessionId: string) =>
        set({ currentSessionId: sessionId }),

      clearCurrentSession: () =>
        set({ currentSessionId: null }),

      addToHistory: (sessionId: string) =>
        set((state) => {
          const currentPinned = state.pinnedSessions ?? [];
          const pinnedSet = new Set(currentPinned);

          if (pinnedSet.has(sessionId)) {
            return {
              pinnedSessions: [
                sessionId,
                ...currentPinned.filter((id) => id !== sessionId),
              ],
            };
          }

          const currentHistory = state.sessionHistory ?? [];
          const recentSessions = currentHistory.filter(
            (id) => id !== sessionId && !pinnedSet.has(id)
          );

          return {
            sessionHistory: [sessionId, ...recentSessions].slice(0, MAX_HISTORY),
          };
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

      togglePinSession: (sessionId: string) =>
        set((state) => {
          const currentPinned = state.pinnedSessions ?? [];
          const currentHistory = state.sessionHistory ?? [];
          const isPinned = currentPinned.includes(sessionId);

          if (isPinned) {
            const updatedPinned = currentPinned.filter(
              (id) => id !== sessionId
            );

            const updatedHistory = [
              sessionId,
              ...currentHistory.filter((id) => id !== sessionId),
            ].slice(0, MAX_HISTORY);

            return {
              pinnedSessions: updatedPinned,
              sessionHistory: updatedHistory,
            };
          }

          const nextPinnedRaw = [
            sessionId,
            ...currentPinned.filter((id) => id !== sessionId),
          ];
          const nextPinned = nextPinnedRaw.slice(0, MAX_PINNED);
          const overflow = nextPinnedRaw.slice(MAX_PINNED);

          const filteredHistory = currentHistory.filter(
            (id) => id !== sessionId && !overflow.includes(id)
          );

          return {
            pinnedSessions: nextPinned,
            sessionHistory: [...overflow, ...filteredHistory].slice(0, MAX_HISTORY),
          };
        }),
    }),
    {
      name: 'alex-session-storage',
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
