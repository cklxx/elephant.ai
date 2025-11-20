// Formatting utilities for agent events
// Extracted from TerminalOutput.tsx for better maintainability

import { AnyAgentEvent } from '@/lib/types';

/**
 * Format tool call arguments for display
 * Handles string, object, and empty arguments
 */
export function formatArgs(args: any): string {
  if (!args) return '';
  if (typeof args === 'string') return args;

  const keys = Object.keys(args);
  if (keys.length === 0) return '';
  if (keys.length === 1) return String(args[keys[0]]);

  return keys.slice(0, 2).map(k => `${k}: ${String(args[k]).slice(0, 20)}`).join(', ');
}

/**
 * Format tool call result for display
 * Handles various result formats (string, object with output/content, etc.)
 */
export function formatResult(result: any): string {
  if (!result) return '';
  if (typeof result === 'string') return result.slice(0, 100);
  if (result.output) return String(result.output).slice(0, 100);
  if (result.content) return String(result.content).slice(0, 100);
  return JSON.stringify(result).slice(0, 100);
}

/**
 * Format event content based on event type
 * Returns human-readable string representation of the event
 */
export function formatContent(event: AnyAgentEvent): string {
  switch (event.event_type) {
    case 'user_task':
      if ('task' in event) {
        return event.task;
      }
      return 'User task';

    case 'task_analysis':
      if ('action_name' in event) {
        const parts: string[] = [event.action_name];
        if ('goal' in event && event.goal) {
          parts.push(event.goal);
        }
        if ('approach' in event && event.approach) {
          parts.push(event.approach);
        }
        return parts.filter(Boolean).join(' · ');
      }
      return 'Task analysis';

    case 'iteration_start':
      if ('iteration' in event) {
        return `Iteration ${event.iteration}/${event.total_iters}`;
      }
      return 'Iteration started';

    case 'thinking':
      return 'Thinking...';

    case 'think_complete':
      if ('content' in event) {
        return event.content;
      }
      return 'Response received';

    case 'tool_call_start':
      if ('tool_name' in event) {
        const preview =
          'arguments_preview' in event && event.arguments_preview
            ? event.arguments_preview
            : formatArgs(event.arguments);
        return preview ? `▸ ${event.tool_name}(${preview})` : `▸ ${event.tool_name}`;
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

    case 'task_cancelled': {
      const requestedBy = 'requested_by' in event ? event.requested_by : undefined;
      const actorPrefix = requestedBy === 'user' ? '⏹ You stopped the agent' : '⏹ Task cancelled';
      const reason =
        'reason' in event && event.reason && event.reason !== 'cancelled'
          ? ` · Reason: ${event.reason}`
          : '';
      return `${actorPrefix}${reason}`;
    }

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

    case 'subagent_progress':
      // Keep delegated subagent display strings even though the backend currently
      // does not emit these event types; retained for UI resilience and tests.
      return `↺ Subagent progress ${event.completed}/${event.total} · ${event.tokens} tokens · ${event.tool_calls} tool calls`;

    case 'subagent_complete':
      // Keep delegated subagent display strings even though the backend currently
      // does not emit these event types; retained for UI resilience and tests.
      return `✓ Subagent summary ${event.success}/${event.total} succeeded (${event.failed} failed, ${event.tokens} tokens, ${event.tool_calls} tool calls)`;

    case 'browser_info':
      if ('message' in event && event.message) {
        return `🧭 Browser diagnostics: ${event.message}`;
      }
      return 'Browser diagnostics captured';

    case 'tool_call_stream':
      if ('chunk' in event) {
        return event.chunk;
      }
      return '';

    default:
      return JSON.stringify(event).slice(0, 100);
  }
}

/**
 * Format timestamp for display
 * Returns HH:MM:SS format
 */
export function formatTimestamp(timestamp?: string | number): string {
  return new Date(timestamp || Date.now()).toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}
