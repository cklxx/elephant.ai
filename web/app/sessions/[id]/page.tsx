'use client';

import { useParams } from 'next/navigation';
import { useSessionDetails } from '@/hooks/useSessionStore';
import { useSSE } from '@/hooks/useSSE';
import { AgentOutput } from '@/components/agent/AgentOutput';
import { TaskInput } from '@/components/agent/TaskInput';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Loader2, ArrowLeft } from 'lucide-react';
import Link from 'next/link';
import { formatRelativeTime } from '@/lib/utils';
import { toast } from '@/components/ui/toast';

export default function SessionDetailsPage() {
  const params = useParams();
  const sessionId = params.id as string;

  const { data: sessionData, isLoading, error } = useSessionDetails(sessionId);
  const { mutate: executeTask, isPending } = useTaskExecution();

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
          toast.success('Task started', 'Execution has begun in this session.');
        },
        onError: (error) => {
          toast.error('Failed to execute task', error.message);
        },
      }
    );
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
        <p>Error loading session: {error.message}</p>
      </div>
    );
  }

  if (!sessionData) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500">
        <p>Session not found</p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Link href="/sessions">
          <Button variant="ghost" size="sm">
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back
          </Button>
        </Link>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold text-gray-900">
              Session Details
            </h1>
            <Badge variant={isConnected ? 'success' : 'default'}>
              {isConnected ? 'Active' : 'Inactive'}
            </Badge>
          </div>
          <p className="text-gray-600 mt-1">
            Session ID: {sessionId}
          </p>
        </div>
      </div>

      {/* Session info */}
      <Card>
        <CardHeader>
          <CardTitle>Session Information</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <p className="text-sm text-gray-600">Created</p>
              <p className="font-medium">
                {formatRelativeTime(sessionData.session.created_at)}
              </p>
            </div>
            <div>
              <p className="text-sm text-gray-600">Last Updated</p>
              <p className="font-medium">
                {formatRelativeTime(sessionData.session.updated_at)}
              </p>
            </div>
            <div>
              <p className="text-sm text-gray-600">Total Tasks</p>
              <p className="font-medium">{sessionData.session.task_count}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Task input */}
      <Card className="p-6">
        <div className="space-y-4">
          <h2 className="text-xl font-semibold text-gray-900">New Task</h2>
          <TaskInput
            onSubmit={handleTaskSubmit}
            disabled={isPending}
            loading={isPending}
          />
        </div>
      </Card>

      {/* Agent output */}
      <AgentOutput
        events={events}
        isConnected={isConnected}
        isReconnecting={isReconnecting}
        error={sseError}
        reconnectAttempts={reconnectAttempts}
        onReconnect={reconnect}
      />

      {/* Task history */}
      {sessionData.tasks && sessionData.tasks.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Task History</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {sessionData.tasks.map((task) => (
                <div
                  key={task.task_id}
                  className="flex items-center justify-between p-3 bg-gray-50 rounded-lg"
                >
                  <div>
                    <p className="font-medium">Task {task.task_id.slice(0, 8)}</p>
                    <p className="text-sm text-gray-500">
                      {formatRelativeTime(task.created_at)}
                    </p>
                  </div>
                  <Badge
                    variant={
                      task.status === 'completed'
                        ? 'success'
                        : task.status === 'failed'
                        ? 'error'
                        : 'info'
                    }
                  >
                    {task.status}
                  </Badge>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
