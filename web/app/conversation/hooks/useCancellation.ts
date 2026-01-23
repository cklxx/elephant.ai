import { useCallback, useEffect, useRef, useState } from "react";

import { useCancelTask } from "@/hooks/useTaskExecution";
import { toast } from "@/components/ui/toast";
import { useI18n } from "@/lib/i18n";
import { captureEvent } from "@/lib/analytics/posthog";
import { AnalyticsEvent } from "@/lib/analytics/events";
import {
  formatParsedError,
  getErrorLogPayload,
  isAPIError,
  parseError,
} from "@/lib/errors";

interface UseCancellationOptions {
  activeTaskId: string | null;
  resolvedSessionId: string | null;
  useMockStream: boolean;
  setActiveTaskId: React.Dispatch<React.SetStateAction<string | null>>;
}

export function useCancellation({
  activeTaskId,
  resolvedSessionId,
  useMockStream,
  setActiveTaskId,
}: UseCancellationOptions) {
  const [cancelRequested, setCancelRequested] = useState(false);
  const cancelIntentRef = useRef(false);
  const activeTaskIdRef = useRef<string | null>(activeTaskId);
  const { t } = useI18n();

  useEffect(() => {
    activeTaskIdRef.current = activeTaskId;
  }, [activeTaskId]);

  const { mutate: cancelTask, isPending: isCancelPending } = useCancelTask();

  const performCancellation = useCallback(
    (taskId: string) => {
      cancelIntentRef.current = false;

      if (useMockStream) {
        setActiveTaskId(null);
        setCancelRequested(false);
        toast.success(
          t("console.toast.taskCancelRequested.title"),
          t("console.toast.taskCancelRequested.description"),
        );
        captureEvent(AnalyticsEvent.TaskCancelRequested, {
          session_id: resolvedSessionId ?? null,
          task_id: taskId,
          status: "success",
          mock_stream: true,
        });
        return;
      }

      cancelTask(taskId, {
        onSuccess: () => {
          const currentActiveTaskId = activeTaskIdRef.current;

          if (!currentActiveTaskId || currentActiveTaskId === taskId) {
            setActiveTaskId((prevActiveTaskId) =>
              prevActiveTaskId === taskId ? null : prevActiveTaskId,
            );
          }
          toast.success(
            t("console.toast.taskCancelRequested.title"),
            t("console.toast.taskCancelRequested.description"),
          );
          captureEvent(AnalyticsEvent.TaskCancelRequested, {
            session_id: resolvedSessionId ?? null,
            task_id: taskId,
            status: "success",
            mock_stream: false,
          });
        },
        onError: (cancelError) => {
          console.error(
            "[ConversationPage] Task cancellation error:",
            getErrorLogPayload(cancelError),
          );
          setCancelRequested(false);
          const parsed = parseError(cancelError, t("common.error.unknown"));
          toast.error(
            t("console.toast.taskCancelError.title"),
            t("console.toast.taskCancelError.description", {
              message: formatParsedError(parsed),
            }),
          );
          captureEvent(AnalyticsEvent.TaskCancelFailed, {
            session_id: resolvedSessionId ?? null,
            task_id: taskId,
            error_kind: isAPIError(cancelError) ? "api" : "unknown",
            ...(isAPIError(cancelError)
              ? { status_code: cancelError.status }
              : {}),
          });
        },
      });
    },
    [
      cancelTask,
      resolvedSessionId,
      setActiveTaskId,
      t,
      useMockStream,
    ],
  );

  const handleStop = useCallback(() => {
    if (isCancelPending) {
      return;
    }

    captureEvent(AnalyticsEvent.TaskCancelRequested, {
      session_id: resolvedSessionId ?? null,
      task_id: activeTaskId ?? null,
      status: "initiated",
      mock_stream: useMockStream,
      request_state: activeTaskId ? "inflight" : "queued",
    });

    setCancelRequested(true);
    if (activeTaskId) {
      performCancellation(activeTaskId);
    } else {
      cancelIntentRef.current = true;
    }
  }, [
    activeTaskId,
    isCancelPending,
    performCancellation,
    resolvedSessionId,
    useMockStream,
  ]);

  return {
    cancelRequested,
    setCancelRequested,
    cancelIntentRef,
    activeTaskIdRef,
    performCancellation,
    handleStop,
    isCancelPending,
  };
}
