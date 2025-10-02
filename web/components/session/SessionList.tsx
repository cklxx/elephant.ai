'use client';

import { SessionCard } from './SessionCard';
import { useSessions, useDeleteSession, useForkSession } from '@/hooks/useSessionStore';
import { Loader2 } from 'lucide-react';

export function SessionList() {
  const { data, isLoading, error } = useSessions();
  const deleteSession = useDeleteSession();
  const forkSession = useForkSession();

  const handleDelete = async (sessionId: string) => {
    if (confirm('Are you sure you want to delete this session?')) {
      await deleteSession.mutateAsync(sessionId);
    }
  };

  const handleFork = async (sessionId: string) => {
    const result = await forkSession.mutateAsync(sessionId);
    if (result) {
      alert(`Session forked successfully! New session ID: ${result.new_session_id}`);
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
  );
}
