'use client';

import { useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AnyAgentEvent } from '@/lib/types';
import { ResearchPlanCard } from './ResearchPlanCard';
import { apiClient } from '@/lib/api';
import { AlertCircle, Loader2, WifiOff } from 'lucide-react';

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

  // Connection status banner
  if (!isConnected || error) {
    return (
      <div className="flex flex-col items-center justify-center h-full space-y-3">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          {isReconnecting ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" />
              <span>Reconnecting... (Attempt {reconnectAttempts})</span>
            </>
          ) : (
            <>
              <WifiOff className="h-4 w-4" />
              <span>Disconnected</span>
            </>
          )}
        </div>

        {error && (
          <div className="flex items-center gap-2 text-xs text-red-500">
            <AlertCircle className="h-3 w-3" />
            <span>{error}</span>
          </div>
        )}

        <button
          onClick={onReconnect}
          className="text-xs px-3 py-1.5 bg-primary text-primary-foreground rounded hover:opacity-90 transition-opacity"
        >
          Reconnect
        </button>
      </div>
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

      {/* Event stream - terminal style */}
      <div className="space-y-2 font-mono text-xs">
        {events.map((event, idx) => (
          <EventLine key={idx} event={event} />
        ))}
      </div>

      {/* Active indicator */}
      {isConnected && events.length > 0 && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground pt-2">
          <div className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />
          <span>Listening for events...</span>
        </div>
      )}
    </div>
  );
}

// Single event line component
function EventLine({ event }: { event: AnyAgentEvent }) {
  const timestamp = new Date(event.timestamp || Date.now()).toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });

  // Event type styling
  const getEventStyle = () => {
    switch (event.event_type) {
      case 'user_task':
        return 'text-blue-700 font-semibold';
      case 'task_analysis':
        return 'text-blue-600';
      case 'task_complete':
        return 'text-green-500 font-semibold';
      case 'error':
        return 'text-red-500 font-semibold';
      case 'research_plan':
        return 'text-blue-600';
      case 'tool_call_start':
        return 'text-cyan-600';
      case 'tool_call_complete':
        return 'text-cyan-500';
      case 'thinking':
        return 'text-purple-600';
      case 'think_complete':
        return 'text-purple-500';
      case 'step_started':
        return 'text-yellow-600';
      case 'step_completed':
        return 'text-yellow-500';
      case 'iteration_start':
        return 'text-gray-600';
      case 'iteration_complete':
        return 'text-gray-500';
      default:
        return 'text-muted-foreground';
    }
  };

  // Format event content
  const formatContent = () => {
    switch (event.event_type) {
      case 'user_task':
        if ('task' in event) {
          return `👤 User: ${event.task}`;
        }
        return 'User task';

      case 'task_analysis':
        if ('action_name' in event) {
          return `📋 ${event.action_name} - ${event.goal}`;
        }
        return 'Task analysis';

      case 'iteration_start':
        if ('iteration' in event) {
          return `→ Iteration ${event.iteration}/${event.total_iters}`;
        }
        return 'Iteration started';

      case 'thinking':
        if ('iteration' in event) {
          return `💭 Thinking... (iteration ${event.iteration})`;
        }
        return '💭 Thinking...';

      case 'think_complete':
        if ('content' in event) {
          const preview = event.content.slice(0, 100);
          const suffix = event.content.length > 100 ? '...' : '';
          return `✓ Response: ${preview}${suffix}`;
        }
        return '✓ Response received';

      case 'tool_call_start':
        if ('tool_name' in event) {
          return `▸ ${event.tool_name}(${formatArgs(event.arguments)})`;
        }
        return 'Tool executing';

      case 'tool_call_complete':
        if ('tool_name' in event) {
          const icon = event.error ? '✗' : '✓';
          const content = event.error || formatResult(event.result);
          return `${icon} ${event.tool_name} → ${content}`;
        }
        return 'Tool complete';

      case 'iteration_complete':
        if ('iteration' in event) {
          return `✓ Iteration ${event.iteration} complete (${event.tokens_used} tokens, ${event.tools_run} tools)`;
        }
        return 'Iteration complete';

      case 'task_complete':
        if ('final_answer' in event) {
          const preview = event.final_answer.slice(0, 150);
          const suffix = event.final_answer.length > 150 ? '...' : '';
          return `✓ Task Complete\n${preview}${suffix}`;
        }
        return '✓ Task complete';

      case 'error':
        if ('error' in event) {
          return `✗ Error: ${event.error}`;
        }
        return '✗ Error occurred';

      case 'research_plan':
        if ('plan_steps' in event) {
          return `→ Research plan created (${event.plan_steps.length} steps, ~${event.estimated_iterations} iterations)`;
        }
        return 'Research plan created';

      case 'step_started':
        if ('step_description' in event) {
          return `→ Step ${event.step_index + 1}: ${event.step_description}`;
        }
        return 'Step started';

      case 'step_completed':
        if ('step_result' in event) {
          return `✓ Step ${event.step_index + 1} complete: ${event.step_result.slice(0, 80)}`;
        }
        return 'Step completed';

      case 'browser_snapshot':
        if ('url' in event) {
          return `📸 Browser snapshot: ${event.url}`;
        }
        return 'Browser snapshot';

      case 'tool_call_stream':
        if ('chunk' in event) {
          return event.chunk;
        }
        return '';

      default:
        return JSON.stringify(event).slice(0, 100);
    }
  };

  return (
    <div className="flex gap-3 group hover:bg-muted/30 -mx-2 px-2 py-1 rounded transition-colors">
      <span className="text-muted-foreground/50 flex-shrink-0 select-none">
        {timestamp}
      </span>
      <span className={getEventStyle()}>
        {formatContent()}
      </span>
    </div>
  );
}

// Helper functions
function formatArgs(args: any): string {
  if (!args) return '';
  if (typeof args === 'string') return args;

  const keys = Object.keys(args);
  if (keys.length === 0) return '';
  if (keys.length === 1) return String(args[keys[0]]);

  return keys.slice(0, 2).map(k => `${k}: ${String(args[k]).slice(0, 20)}`).join(', ');
}

function formatResult(result: any): string {
  if (!result) return '';
  if (typeof result === 'string') return result.slice(0, 100);
  if (result.output) return String(result.output).slice(0, 100);
  if (result.content) return String(result.content).slice(0, 100);
  return JSON.stringify(result).slice(0, 100);
}
