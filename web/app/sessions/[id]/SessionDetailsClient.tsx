'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { ArrowLeft, Loader2 } from 'lucide-react';
import { useSessionDetails } from '@/hooks/useSessionStore';
import { useSSE } from '@/hooks/useSSE';
import { AgentOutput } from '@/components/agent/AgentOutput';
import { TaskInput } from '@/components/agent/TaskInput';
import { useTaskExecution, useCancelTask } from '@/hooks/useTaskExecution';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { formatRelativeTime } from '@/lib/utils';
import { toast } from '@/components/ui/toast';
import { getLanguageLocale, useI18n, type TranslationKey } from '@/lib/i18n';
import { formatParsedError, getErrorLogPayload, parseError } from '@/lib/errors';
import type { AnyAgentEvent, AttachmentUpload } from '@/lib/types';
import { eventMatches } from '@/lib/types';

const statusLabels: Record<string, TranslationKey> = {
  completed: 'sessions.details.history.status.completed',
  running: 'sessions.details.history.status.running',
  pending: 'sessions.details.history.status.pending',
  failed: 'sessions.details.history.status.failed',
  in_progress: 'sessions.details.history.status.in_progress',
  error: 'sessions.details.history.status.error',
  cancelled: 'sessions.details.history.status.cancelled',
};

type SessionDetailsClientProps = {
  sessionId: string;
};

export function SessionDetailsClient({ sessionId }: SessionDetailsClientProps) {
  const { data: sessionData, isLoading, error } = useSessionDetails(sessionId);
  const { t, language } = useI18n();
  const locale = getLanguageLocale(language);

  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [cancelRequested, setCancelRequested] = useState(false);
  const cancelIntentRef = useRef(false);
  const activeTaskIdRef = useRef<string | null>(null);

  useEffect(() => {
    activeTaskIdRef.current = activeTaskId;
  }, [activeTaskId]);

  const { mutate: cancelTask, isPending: isCancelPending } = useCancelTask();

  const performCancellation = useCallback(
    (taskId: string) => {
      cancelIntentRef.current = false;
      cancelTask(taskId, {
        onSuccess: () => {
          const currentActiveTaskId = activeTaskIdRef.current;

          if (!currentActiveTaskId || currentActiveTaskId === taskId) {
            setActiveTaskId((prevActiveTaskId) =>
              prevActiveTaskId === taskId ? null : prevActiveTaskId
            );
            setCancelRequested(false);
          }
          toast.success(
            t('sessions.details.toast.taskCancelRequested.title'),
            t('sessions.details.toast.taskCancelRequested.description')
          );
        },
        onError: (cancelError) => {
          console.error(
            '[SessionDetails] Task cancellation error:',
            getErrorLogPayload(cancelError)
          );
          setCancelRequested(false);
          const parsed = parseError(cancelError, t('common.error.unknown'));
          toast.error(
            t('sessions.details.toast.taskCancelError.title'),
            t('sessions.details.toast.taskCancelError.description', {
              message: formatParsedError(parsed),
            })
          );
        },
      });
    },
    [cancelTask, t]
  );

  const { mutate: executeTask, isPending: isCreatePending } = useTaskExecution({
    onSuccess: (response) => {
      setActiveTaskId(response.task_id);
      if (cancelIntentRef.current) {
        setCancelRequested(true);
        performCancellation(response.task_id);
      }
    },
    onError: (submitError) => {
      cancelIntentRef.current = false;
      setCancelRequested(false);
      console.error(
        '[SessionDetails] Task execution error:',
        getErrorLogPayload(submitError)
      );
      const parsed = parseError(submitError, t('common.error.unknown'));
      toast.error(
        t('sessions.details.toast.taskError.title'),
        t('sessions.details.toast.taskError.description', {
          message: formatParsedError(parsed),
        })
      );
    },
  });

  const handleTaskSubmit = useCallback(
    (task: string, attachments: AttachmentUpload[]) => {
      cancelIntentRef.current = false;
      setCancelRequested(false);
      executeTask({
        task,
        session_id: sessionId,
        attachments: attachments.length ? attachments : undefined,
      });
    },
    [executeTask, sessionId]
  );

  const handleStop = useCallback(() => {
    if (isCancelPending) {
      return;
    }
    setCancelRequested(true);
    if (activeTaskId) {
      performCancellation(activeTaskId);
    } else {
      cancelIntentRef.current = true;
    }
  }, [activeTaskId, isCancelPending, performCancellation]);

  const handleAgentEvent = useCallback(
    (event: AnyAgentEvent) => {
      const currentId = activeTaskIdRef.current;
      if (!currentId || !event.task_id || event.task_id !== currentId) {
        return;
      }
      if (
        eventMatches(event, 'workflow.result.final', 'task_complete') ||
        eventMatches(event, 'workflow.result.cancelled', 'task_cancelled') ||
        eventMatches(event, 'workflow.node.failed', 'error')
      ) {
        setActiveTaskId(null);
        setCancelRequested(false);
        cancelIntentRef.current = false;
      }
    },
    [setActiveTaskId, setCancelRequested]
  );

  const {
    events,
    isConnected,
    isReconnecting,
    error: sseError,
    reconnectAttempts,
    reconnect,
  } = useSSE(sessionId, { onEvent: handleAgentEvent });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64" aria-live="polite">
        <Loader2 className="h-8 w-8 animate-spin text-primary" aria-hidden />
        <span className="sr-only">{t('sessions.details.loading')}</span>
      </div>
    );
  }

  if (error) {
    const parsed = parseError(error, t('common.error.unknown'));
    return (
      <div className="flex items-center justify-center h-64 text-destructive">
        <p>{t('sessions.details.error', { message: formatParsedError(parsed) })}</p>
      </div>
    );
  }

  if (!sessionData) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500">
        <p>{t('sessions.details.notFound')}</p>
      </div>
    );
  }

  const isTaskRunning = Boolean(activeTaskId);
  const inputDisabled =
    isCreatePending || isTaskRunning || cancelRequested || isCancelPending;
  const stopPending = cancelRequested || isCancelPending;

  return (
    <div className="space-y-8">
      <div className="flex items-center gap-4">
        <Link href="/sessions">
          <Button variant="ghost" size="sm">
            <ArrowLeft className="h-4 w-4 mr-2" />
            {t('sessions.details.back')}
          </Button>
        </Link>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold text-gray-900">{t('sessions.details.title')}</h1>
            <Badge variant={isConnected ? 'success' : 'default'}>
              {isConnected ? t('sessions.details.status.active') : t('sessions.details.status.inactive')}
            </Badge>
          </div>
          <p className="text-gray-600 mt-1">{t('sessions.details.sessionId', { id: sessionId })}</p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('sessions.details.info.title')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <p className="text-sm text-gray-600">{t('sessions.details.info.created')}</p>
              <p className="font-medium">{formatRelativeTime(sessionData.session.created_at, locale)}</p>
            </div>
            <div>
              <p className="text-sm text-gray-600">{t('sessions.details.info.updated')}</p>
              <p className="font-medium">{formatRelativeTime(sessionData.session.updated_at, locale)}</p>
            </div>
            <div>
              <p className="text-sm text-gray-600">{t('sessions.details.info.taskCount')}</p>
              <p className="font-medium">{sessionData.session.task_count}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="p-6">
        <div className="space-y-4">
          <h2 className="text-xl font-semibold text-gray-900">{t('sessions.details.newTask')}</h2>
          <TaskInput
            onSubmit={handleTaskSubmit}
            disabled={inputDisabled}
            loading={isCreatePending}
            isRunning={isTaskRunning}
            onStop={handleStop}
            stopPending={stopPending}
            stopDisabled={isCancelPending}
          />
        </div>
      </Card>

      <AgentOutput
        events={events}
        isConnected={isConnected}
        isReconnecting={isReconnecting}
        error={sseError}
        reconnectAttempts={reconnectAttempts}
        onReconnect={reconnect}
      />

      {sessionData.tasks && sessionData.tasks.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t('sessions.details.history')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {sessionData.tasks.map((task) => {
                const statusKey = statusLabels[task.status];
                const translatedStatus = statusKey ? t(statusKey) : task.status.toUpperCase();

                const badgeVariant =
                  task.status === 'completed'
                    ? 'success'
                    : task.status === 'failed' || task.status === 'error'
                      ? 'error'
                      : task.status === 'cancelled'
                        ? 'warning'
                        : 'info';

                return (
                  <div key={task.task_id} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                    <div>
                      <p className="font-medium text-gray-900">{translatedStatus}</p>
                      <p className="text-sm text-gray-600">
                        {t('sessions.details.history.started', {
                          time: formatRelativeTime(task.created_at, locale),
                        })}
                      </p>
                      <p className="text-xs text-gray-500 mt-1 font-mono">
                        {`Task: ${task.task_id}`}
                        {task.parent_task_id ? ` · Parent: ${task.parent_task_id}` : ''}
                      </p>
                    </div>
                    <Badge variant={badgeVariant}>
                      {translatedStatus}
                    </Badge>
                  </div>
                );
              })}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
