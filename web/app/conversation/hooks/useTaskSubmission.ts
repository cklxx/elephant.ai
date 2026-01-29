import { useCallback } from "react";

import { useTaskExecution } from "@/hooks/useTaskExecution";
import { captureEvent } from "@/lib/analytics/posthog";
import { AnalyticsEvent } from "@/lib/analytics/events";
import {
  formatParsedError,
  getErrorLogPayload,
  isAPIError,
  parseError,
} from "@/lib/errors";
import { toast } from "@/components/ui/toast";
import { useI18n } from "@/lib/i18n";
import type { AnyAgentEvent, AttachmentPayload, AttachmentUpload } from "@/lib/types";

interface UseTaskSubmissionOptions {
  useMockStream: boolean;
  resolvedSessionId: string | null;
  sessionId: string | null;
  currentSessionId: string | null;
  prewarmSessionId: string | null;
  prefillTask: string | null;
  setSessionId: React.Dispatch<React.SetStateAction<string | null>>;
  setTaskId: React.Dispatch<React.SetStateAction<string | null>>;
  setActiveTaskId: React.Dispatch<React.SetStateAction<string | null>>;
  setPrewarmSessionId: React.Dispatch<React.SetStateAction<string | null>>;
  setCancelRequested: React.Dispatch<React.SetStateAction<boolean>>;
  cancelIntentRef: React.MutableRefObject<boolean>;
  addEvent: (event: AnyAgentEvent) => void;
  clearEvents: () => void;
  setCurrentSession: (id: string) => void;
  addToHistory: (id: string) => void;
  clearCurrentSession: () => void;
  removeSession: (id: string) => void;
  performCancellation: (taskId: string) => void;
  pendingSubmitTsRef: React.MutableRefObject<number | null>;
  submittedAtByTaskRef: React.MutableRefObject<Map<string, number>>;
  firstTokenReportedRef: React.MutableRefObject<Set<string>>;
}

export function useTaskSubmission({
  useMockStream,
  resolvedSessionId,
  sessionId,
  currentSessionId,
  prewarmSessionId,
  prefillTask,
  setSessionId,
  setTaskId,
  setActiveTaskId,
  setPrewarmSessionId,
  setCancelRequested,
  cancelIntentRef,
  addEvent,
  clearEvents,
  setCurrentSession,
  addToHistory,
  clearCurrentSession,
  removeSession,
  performCancellation,
  pendingSubmitTsRef,
  submittedAtByTaskRef,
  firstTokenReportedRef,
}: UseTaskSubmissionOptions) {
  const { t } = useI18n();
  const { mutate: executeTask, isPending: isCreatePending } = useTaskExecution();

  const buildAttachmentMap = useCallback(
    (uploads: AttachmentUpload[]) =>
      uploads.reduce<Record<string, AttachmentPayload>>((acc, att) => {
        const { name, ...rest } = att;

        acc[name] = {
          name,
          ...rest,
        } as AttachmentPayload;
        return acc;
      }, {}),
    [],
  );

  const handleTaskSubmit = useCallback(
    (task: string, attachments: AttachmentUpload[]) => {
      console.log("[ConversationPage] Task submitted:", { task, attachments });
      pendingSubmitTsRef.current = performance.now();

      captureEvent(AnalyticsEvent.TaskSubmitted, {
        session_id: resolvedSessionId ?? null,
        has_active_session: Boolean(resolvedSessionId),
        attachment_count: attachments.length,
        has_attachments: attachments.length > 0,
        input_length: task.length,
        mock_stream: useMockStream,
        prefill_present: Boolean(prefillTask),
      });

      cancelIntentRef.current = false;
      setCancelRequested(false);

      if (useMockStream) {
        const submissionTimestamp = new Date();
        const provisionalSessionId =
          sessionId ||
          currentSessionId ||
          `mock-${submissionTimestamp.getTime().toString(36)}`;
        const mockTaskId = `mock-task-${submissionTimestamp.getTime().toString(36)}`;

        const attachmentMap = buildAttachmentMap(attachments);

        addEvent({
          event_type: "workflow.input.received",
          timestamp: submissionTimestamp.toISOString(),
          agent_level: "core",
          session_id: provisionalSessionId,
          run_id: mockTaskId,
          task,
          attachments: Object.keys(attachmentMap).length
            ? attachmentMap
            : undefined,
        });

        setSessionId(provisionalSessionId);
        setTaskId(mockTaskId);
        setActiveTaskId(mockTaskId);
        setCurrentSession(provisionalSessionId);
        addToHistory(provisionalSessionId);
        return;
      }

      const initialSessionId = resolvedSessionId ?? prewarmSessionId;
      let retriedWithoutSession = false;

      const runExecution = (requestedSessionId: string | null) => {
        executeTask(
          {
            task,
            session_id: requestedSessionId ?? undefined,
            attachments: attachments.length ? attachments : undefined,
          },
          {
            onSuccess: (data) => {
              console.log("[ConversationPage] Task execution started:", data);
              setPrewarmSessionId(null);
              setSessionId(data.session_id);
              setTaskId(data.task_id);
              setActiveTaskId(data.task_id);
              setCurrentSession(data.session_id);
              addToHistory(data.session_id);

              const submitTs = pendingSubmitTsRef.current;
              if (typeof submitTs === "number") {
                submittedAtByTaskRef.current.set(data.task_id, submitTs);
                firstTokenReportedRef.current.delete(data.task_id);
              }

              const attachmentMap = buildAttachmentMap(attachments);
              addEvent({
                event_type: "workflow.input.received",
                timestamp: new Date().toISOString(),
                agent_level: "core",
                session_id: data.session_id,
                run_id: data.task_id,
                parent_run_id: data.parent_task_id ?? undefined,
                task,
                attachments: Object.keys(attachmentMap).length
                  ? attachmentMap
                  : undefined,
              });
              if (cancelIntentRef.current) {
                setCancelRequested(true);
                performCancellation(data.task_id);
              }
            },
            onError: (error) => {
              const isStaleSession =
                !retriedWithoutSession &&
                !!requestedSessionId &&
                isAPIError(error) &&
                error.status === 404;

              if (isStaleSession) {
                retriedWithoutSession = true;
                console.warn(
                  "[ConversationPage] Session not found, retrying without session_id",
                  {
                    sessionId: requestedSessionId,
                    error: getErrorLogPayload(error),
                  },
                );

                setSessionId(null);
                setTaskId(null);
                setActiveTaskId(null);
                setCancelRequested(false);
                cancelIntentRef.current = false;
                clearCurrentSession();
                removeSession(requestedSessionId);
                clearEvents();

                captureEvent(AnalyticsEvent.TaskRetriedWithoutSession, {
                  session_id: requestedSessionId,
                  error_status: 404,
                  mock_stream: useMockStream,
                });

                runExecution(null);
                return;
              }

              console.error(
                "[ConversationPage] Task execution error:",
                getErrorLogPayload(error),
              );
              cancelIntentRef.current = false;
              setCancelRequested(false);
              setActiveTaskId(null);
              const parsed = parseError(error, t("common.error.unknown"));
              toast.error(t("console.toast.taskFailed"), formatParsedError(parsed));
              captureEvent(AnalyticsEvent.TaskSubmissionFailed, {
                session_id: requestedSessionId ?? null,
                is_api_error: isAPIError(error),
                mock_stream: useMockStream,
                ...(isAPIError(error) ? { status_code: error.status } : {}),
              });
            },
          },
        );
      };

      runExecution(initialSessionId ?? null);
    },
    [
      addEvent,
      addToHistory,
      buildAttachmentMap,
      cancelIntentRef,
      clearCurrentSession,
      clearEvents,
      currentSessionId,
      executeTask,
      firstTokenReportedRef,
      pendingSubmitTsRef,
      performCancellation,
      prefillTask,
      prewarmSessionId,
      removeSession,
      resolvedSessionId,
      sessionId,
      setActiveTaskId,
      setCancelRequested,
      setCurrentSession,
      setPrewarmSessionId,
      setSessionId,
      setTaskId,
      submittedAtByTaskRef,
      t,
      useMockStream,
    ],
  );

  return { handleTaskSubmit, isCreatePending };
}
