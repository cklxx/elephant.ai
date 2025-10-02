// Zustand store for session management

import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface SessionState {
  currentSessionId: string | null;
  sessionHistory: string[];
  setCurrentSession: (sessionId: string) => void;
  clearCurrentSession: () => void;
  addToHistory: (sessionId: string) => void;
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      currentSessionId: null,
      sessionHistory: [],

      setCurrentSession: (sessionId: string) =>
        set({ currentSessionId: sessionId }),

      clearCurrentSession: () =>
        set({ currentSessionId: null }),

      addToHistory: (sessionId: string) =>
        set((state) => ({
          sessionHistory: [
            sessionId,
            ...state.sessionHistory.filter((id) => id !== sessionId),
          ].slice(0, 10), // Keep last 10 sessions
        })),
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
