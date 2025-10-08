'use client';

import { SessionCard } from './SessionCard';
import { useSessions, useDeleteSession, useForkSession } from '@/hooks/useSessionStore';
import { Loader2 } from 'lucide-react';
import { toast } from '@/components/ui/toast';
import { useConfirmDialog } from '@/components/ui/dialog';
import { useI18n } from '@/lib/i18n';

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
        toast.error(
          t('sessions.list.toast.deleteError.title'),
          err instanceof Error
            ? t('sessions.list.toast.deleteError.description', { message: err.message })
            : t('sessions.list.toast.deleteError.description', { message: t('common.error.unknown') })
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
      toast.error(
        t('sessions.list.toast.forkError.title'),
        err instanceof Error
          ? t('sessions.list.toast.forkError.description', { message: err.message })
          : t('sessions.list.toast.forkError.description', { message: t('common.error.unknown') })
      );
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64" aria-live="polite">
        <Loader2 className="h-8 w-8 animate-spin text-blue-600" aria-hidden />
        <span className="sr-only">{t('sessions.list.loading')}</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-64 text-red-600">
        <p>{t('sessions.list.error', { message: error.message })}</p>
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
