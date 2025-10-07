'use client';

import { useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AnyAgentEvent } from '@/lib/types';
import { ResearchPlanCard } from './ResearchPlanCard';
import { ConnectionBanner } from './ConnectionBanner';
import { EventList } from './EventList';
import { apiClient } from '@/lib/api';

interface TerminalOutputProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
  sessionId: string | null;
  taskId: string | null;
}

export function TerminalOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
  sessionId,
  taskId,
}: TerminalOutputProps) {
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Simple approve plan mutation
  const { mutate: approvePlan } = useMutation({
    mutationFn: async ({ sessionId, taskId }: { sessionId: string; taskId: string }) => {
      return apiClient.approvePlan({
        session_id: sessionId,
        task_id: taskId,
        approved: true,
      });
    },
  });

  // Parse plan state from events
  const { planState, currentPlan } = useMemo(() => {
    const lastPlanEvent = [...events]
      .reverse()
      .find((e) => e.event_type === 'research_plan');

    if (!lastPlanEvent || !('plan_steps' in lastPlanEvent)) {
      return { planState: null, currentPlan: null };
    }

    return {
      planState: 'awaiting_approval' as const,
      currentPlan: {
        goal: 'Research task',
        steps: lastPlanEvent.plan_steps,
        estimated_tools: [],
        estimated_iterations: lastPlanEvent.estimated_iterations,
      },
    };
  }, [events]);

  const handleApprove = () => {
    if (!sessionId || !taskId) return;

    setIsSubmitting(true);
    approvePlan(
      { sessionId, taskId },
      {
        onSuccess: () => {
          setIsSubmitting(false);
        },
        onError: () => {
          setIsSubmitting(false);
        },
      }
    );
  };

  // Show connection banner if disconnected
  if (!isConnected || error) {
    return (
      <ConnectionBanner
        isConnected={isConnected}
        isReconnecting={isReconnecting}
        error={error}
        reconnectAttempts={reconnectAttempts}
        onReconnect={onReconnect}
      />
    );
  }

  return (
    <div className="space-y-3">
      {/* Plan approval card - if awaiting */}
      {planState === 'awaiting_approval' && currentPlan && (
        <div className="mb-4">
          <ResearchPlanCard
            plan={currentPlan}
            loading={isSubmitting}
            onApprove={handleApprove}
          />
        </div>
      )}

      {/* Event stream - with virtual scrolling */}
      <EventList events={events} isConnected={isConnected} />
    </div>
  );
}
