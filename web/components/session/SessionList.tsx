'use client';

import { SessionCard } from './SessionCard';
import { useSessions, useDeleteSession, useForkSession } from '@/hooks/useSessionStore';
import { Loader2 } from 'lucide-react';
import { toast } from '@/components/ui/toast';
import { useConfirmDialog } from '@/components/ui/dialog';
import { useI18n } from '@/lib/i18n';
import { formatParsedError, getErrorLogPayload, parseError } from '@/lib/errors';

export function SessionList() {
  const { data, isLoading, error } = useSessions();
  const deleteSession = useDeleteSession();
  const forkSession = useForkSession();
  const { confirm, ConfirmDialog } = useConfirmDialog();
  const { t } = useI18n();

  const handleDelete = async (sessionId: string) => {
    const confirmed = await confirm({
      title: t('sessions.list.confirmDelete.title'),
      description: t('sessions.list.confirmDelete.description'),
      confirmText: t('sessions.list.confirmDelete.confirm'),
      cancelText: t('sessions.list.confirmDelete.cancel'),
      variant: 'danger',
    });

    if (confirmed) {
      try {
        await deleteSession.mutateAsync(sessionId);
        toast.success(
          t('sessions.list.toast.deleteSuccess.title'),
          t('sessions.list.toast.deleteSuccess.description')
        );
      } catch (err) {
        console.error(
          '[SessionList] Failed to delete session:',
          getErrorLogPayload(err)
        );
        const parsed = parseError(err, t('common.error.unknown'));
        toast.error(
          t('sessions.list.toast.deleteError.title'),
          t('sessions.list.toast.deleteError.description', {
            message: formatParsedError(parsed),
          })
        );
      }
    }
  };

  const handleFork = async (sessionId: string) => {
    try {
      const result = await forkSession.mutateAsync(sessionId);
      if (result) {
        toast.success(
          t('sessions.list.toast.forkSuccess.title'),
          t('sessions.list.toast.forkSuccess.description', {
            id: result.new_session_id.slice(0, 8),
          })
        );
      }
    } catch (err) {
      console.error(
        '[SessionList] Failed to fork session:',
        getErrorLogPayload(err)
      );
      const parsed = parseError(err, t('common.error.unknown'));
      toast.error(
        t('sessions.list.toast.forkError.title'),
        t('sessions.list.toast.forkError.description', {
          message: formatParsedError(parsed),
        })
      );
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64" aria-live="polite">
        <Loader2 className="h-8 w-8 animate-spin text-primary" aria-hidden />
        <span className="sr-only">{t('sessions.list.loading')}</span>
      </div>
    );
  }

  if (error) {
    const parsed = parseError(error, t('common.error.unknown'));
    return (
      <div className="flex items-center justify-center h-64 text-destructive">
        <p>{t('sessions.list.error', { message: formatParsedError(parsed) })}</p>
      </div>
    );
  }

  if (!data || data.sessions.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500">
        <p>{t('sessions.list.empty')}</p>
      </div>
    );
  }

  return (
    <>
      <ConfirmDialog />
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {data.sessions.map((session) => (
          <SessionCard
            key={session.id}
            session={session}
            onDelete={handleDelete}
            onFork={handleFork}
          />
        ))}
      </div>
    </>
  );
}
