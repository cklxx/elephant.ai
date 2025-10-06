'use client';

import { SessionCard } from './SessionCard';
import { useSessions, useDeleteSession, useForkSession } from '@/hooks/useSessionStore';
import { Loader2 } from 'lucide-react';
import { toast } from '@/components/ui/toast';
import { useConfirmDialog } from '@/components/ui/dialog';

export function SessionList() {
  const { data, isLoading, error } = useSessions();
  const deleteSession = useDeleteSession();
  const forkSession = useForkSession();
  const { confirm, ConfirmDialog } = useConfirmDialog();

  const handleDelete = async (sessionId: string) => {
    const confirmed = await confirm({
      title: 'Delete Session?',
      description: 'This action cannot be undone. All session data will be permanently deleted.',
      confirmText: 'Delete',
      cancelText: 'Cancel',
      variant: 'danger',
    });

    if (confirmed) {
      try {
        await deleteSession.mutateAsync(sessionId);
        toast.success('Session deleted', 'The session has been permanently removed.');
      } catch (err) {
        toast.error('Failed to delete session', err instanceof Error ? err.message : 'Unknown error');
      }
    }
  };

  const handleFork = async (sessionId: string) => {
    try {
      const result = await forkSession.mutateAsync(sessionId);
      if (result) {
        toast.success(
          'Session forked successfully!',
          `New session ID: ${result.new_session_id.slice(0, 8)}...`
        );
      }
    } catch (err) {
      toast.error('Failed to fork session', err instanceof Error ? err.message : 'Unknown error');
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-blue-600" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-64 text-red-600">
        <p>Error loading sessions: {error.message}</p>
      </div>
    );
  }

  if (!data || data.sessions.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500">
        <p>No sessions found. Create a new task to start a session.</p>
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
