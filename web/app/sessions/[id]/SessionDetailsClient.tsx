'use client';

import Link from 'next/link';
import { ArrowLeft, Loader2 } from 'lucide-react';
import { useSessionDetails } from '@/hooks/useSessionStore';
import { useSSE } from '@/hooks/useSSE';
import { AgentOutput } from '@/components/agent/AgentOutput';
import { TaskInput } from '@/components/agent/TaskInput';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { formatRelativeTime } from '@/lib/utils';
import { toast } from '@/components/ui/toast';
import { getLanguageLocale, useI18n, type TranslationKey } from '@/lib/i18n';

const statusLabels: Record<string, TranslationKey> = {
  completed: 'sessions.details.history.status.completed',
  running: 'sessions.details.history.status.running',
  pending: 'sessions.details.history.status.pending',
  failed: 'sessions.details.history.status.failed',
  in_progress: 'sessions.details.history.status.in_progress',
  error: 'sessions.details.history.status.error',
};

type SessionDetailsClientProps = {
  sessionId: string;
};

export function SessionDetailsClient({ sessionId }: SessionDetailsClientProps) {
  const { data: sessionData, isLoading, error } = useSessionDetails(sessionId);
  const { mutate: executeTask, isPending } = useTaskExecution();
  const { t, language } = useI18n();
  const locale = getLanguageLocale(language);

  const {
    events,
    isConnected,
    isReconnecting,
    error: sseError,
    reconnectAttempts,
    reconnect,
  } = useSSE(sessionId);

  const handleTaskSubmit = (task: string) => {
    executeTask(
      {
        task,
        session_id: sessionId,
      },
      {
        onSuccess: () => {
          toast.success(
            t('sessions.details.toast.taskStarted.title'),
            t('sessions.details.toast.taskStarted.description')
          );
        },
        onError: (submitError) => {
          toast.error(
            t('sessions.details.toast.taskError.title'),
            t('sessions.details.toast.taskError.description', { message: submitError.message })
          );
        },
      }
    );
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64" aria-live="polite">
        <Loader2 className="h-8 w-8 animate-spin text-primary" aria-hidden />
        <span className="sr-only">{t('sessions.details.loading')}</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-64 text-destructive">
        <p>{t('sessions.details.error', { message: error.message })}</p>
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
          <TaskInput onSubmit={handleTaskSubmit} disabled={isPending} loading={isPending} />
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

                return (
                  <div key={task.task_id} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                    <div>
                      <p className="font-medium text-gray-900">{translatedStatus}</p>
                      <p className="text-sm text-gray-600">
                        {t('sessions.details.history.started', {
                          time: formatRelativeTime(task.created_at, locale),
                        })}
                      </p>
                    </div>
                    <Badge variant={task.status === 'completed' ? 'success' : 'info'}>
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
