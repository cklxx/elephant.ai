import { useCallback, useState } from "react";

import { useDeleteSession } from "@/hooks/useSessionStore";
import { toast } from "@/components/ui/toast";
import { useI18n } from "@/lib/i18n";
import {
  formatParsedError,
  getErrorLogPayload,
  isAPIError,
  parseError,
} from "@/lib/errors";
import { captureEvent } from "@/lib/analytics/posthog";
import { AnalyticsEvent } from "@/lib/analytics/events";

interface UseDeleteDialogOptions {
  resolvedSessionId: string | null;
  setSessionId: React.Dispatch<React.SetStateAction<string | null>>;
  setTaskId: React.Dispatch<React.SetStateAction<string | null>>;
  setActiveTaskId: React.Dispatch<React.SetStateAction<string | null>>;
  setCancelRequested: React.Dispatch<React.SetStateAction<boolean>>;
  cancelIntentRef: React.MutableRefObject<boolean>;
  clearEvents: () => void;
  clearCurrentSession: () => void;
  removeSession: (id: string) => void;
}

export function useDeleteDialog({
  resolvedSessionId,
  setSessionId,
  setTaskId,
  setActiveTaskId,
  setCancelRequested,
  cancelIntentRef,
  clearEvents,
  clearCurrentSession,
  removeSession,
}: UseDeleteDialogOptions) {
  const { t } = useI18n();
  const deleteSessionMutation = useDeleteSession();
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);
  const [deleteInProgress, setDeleteInProgress] = useState(false);

  const handleSessionDeleteRequest = useCallback((id: string) => {
    setDeleteTargetId(id);
  }, []);

  const handleDeleteCancel = useCallback(() => {
    if (deleteInProgress) return;
    setDeleteTargetId(null);
  }, [deleteInProgress]);

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteTargetId) return;
    setDeleteInProgress(true);
    try {
      await deleteSessionMutation.mutateAsync(deleteTargetId);
      removeSession(deleteTargetId);
      if (resolvedSessionId === deleteTargetId) {
        clearEvents();
        setSessionId(null);
        setTaskId(null);
        setActiveTaskId(null);
        setCancelRequested(false);
        cancelIntentRef.current = false;
        clearCurrentSession();
      }
      toast.success(t("sidebar.session.toast.deleteSuccess"));
      captureEvent(AnalyticsEvent.SessionDeleted, {
        session_id: deleteTargetId,
        status: "success",
      });
      setDeleteTargetId(null);
    } catch (err) {
      console.error(
        "[ConversationPage] Failed to delete session:",
        getErrorLogPayload(err),
      );
      const parsed = parseError(err, t("common.error.unknown"));
      toast.error(
        t("sidebar.session.toast.deleteError"),
        formatParsedError(parsed),
      );
      captureEvent(AnalyticsEvent.SessionDeleted, {
        session_id: deleteTargetId,
        status: "error",
        error_kind: isAPIError(err) ? "api" : "unknown",
        ...(isAPIError(err) ? { status_code: err.status } : {}),
      });
    } finally {
      setDeleteInProgress(false);
    }
  }, [
    cancelIntentRef,
    clearCurrentSession,
    clearEvents,
    deleteSessionMutation,
    deleteTargetId,
    removeSession,
    resolvedSessionId,
    setActiveTaskId,
    setCancelRequested,
    setSessionId,
    setTaskId,
    t,
  ]);

  return {
    deleteTargetId,
    deleteInProgress,
    handleSessionDeleteRequest,
    handleDeleteCancel,
    handleConfirmDelete,
  };
}
