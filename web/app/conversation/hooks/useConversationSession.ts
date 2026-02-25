import { useCallback, useEffect, useMemo, useState } from "react";

import { useSessionStore } from "@/hooks/useSessionStore";
import { apiClient } from "@/lib/api";
import { captureEvent } from "@/lib/analytics/posthog";
import { AnalyticsEvent } from "@/lib/analytics/events";

interface ConversationSessionActions {
  clearEvents: () => void;
  resetTaskState: () => void;
}

interface UseConversationSessionOptions {
  useMockStream: boolean;
}

export function useConversationSession({ useMockStream }: UseConversationSessionOptions) {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [prewarmSessionId, setPrewarmSessionId] = useState<string | null>(null);

  const {
    currentSessionId,
    setCurrentSession,
    addToHistory,
    clearCurrentSession,
    removeSession,
    sessionHistory = [],
    sessionLabels = {},
  } = useSessionStore();

  const resolvedSessionId = sessionId || currentSessionId;
  const streamSessionId = resolvedSessionId ?? prewarmSessionId;

  const formatSessionBadge = useCallback(
    (value: string) =>
      value.length > 8 ? `${value.slice(0, 4)}…${value.slice(-4)}` : value,
    [],
  );

  useEffect(() => {
    if (useMockStream) return;
    if (resolvedSessionId) return;
    if (prewarmSessionId) return;

    let cancelled = false;
    apiClient
      .createSession()
      .then(({ session_id }) => {
        if (cancelled) return;
        setPrewarmSessionId(session_id);
      })
      .catch((err) => {
        console.warn("[ConversationPage] Failed to prewarm session:", err);
      });

    return () => {
      cancelled = true;
    };
  }, [prewarmSessionId, resolvedSessionId, useMockStream]);

  const handleNewSession = useCallback(({ clearEvents, resetTaskState }: ConversationSessionActions) => {
    setSessionId(null);
    setPrewarmSessionId(null);
    resetTaskState();
    clearEvents();
    clearCurrentSession();
    captureEvent(AnalyticsEvent.SessionCreated, {
      previous_session_id: resolvedSessionId ?? null,
      had_active_session: Boolean(resolvedSessionId),
      history_count: sessionHistory.length,
    });
  }, [clearCurrentSession, resolvedSessionId, sessionHistory.length]);

  const handleSessionSelect = useCallback(
    (id: string, { clearEvents, resetTaskState }: ConversationSessionActions) => {
      if (!id || id === resolvedSessionId) return;
      clearEvents();
      setPrewarmSessionId(null);
      setSessionId(id);
      resetTaskState();
      setCurrentSession(id);
      addToHistory(id);
      captureEvent(AnalyticsEvent.SessionSelected, {
        session_id: id,
        previous_session_id: resolvedSessionId ?? null,
        was_in_history: sessionHistory.includes(id),
      });
    },
    [addToHistory, resolvedSessionId, sessionHistory, setCurrentSession],
  );

  return {
    sessionId,
    setSessionId,
    prewarmSessionId,
    setPrewarmSessionId,
    resolvedSessionId,
    streamSessionId,
    formatSessionBadge,
    handleNewSession,
    handleSessionSelect,
    currentSessionId,
    setCurrentSession,
    addToHistory,
    clearCurrentSession,
    removeSession,
    sessionHistory,
    sessionLabels,
  };
}
