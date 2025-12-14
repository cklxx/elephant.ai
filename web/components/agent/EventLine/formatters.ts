// Formatting utilities for agent events
// Extracted from TerminalOutput.tsx for better maintainability

import { AnyAgentEvent, eventMatches } from '@/lib/types';

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
  switch (true) {
    case event.event_type === 'workflow.input.received':
      if ('task' in event) {
        return event.task;
      }
      return 'User task';

    case eventMatches(event, 'workflow.node.started', 'workflow.node.started') &&
      typeof (event as any).iteration === 'number': {
      return `Iteration ${(event as any).iteration}/${(event as any).total_iters}`;
    }

    case eventMatches(event, 'workflow.node.output.delta', 'workflow.node.output.delta'):
      return eventMatches(event, 'workflow.node.output.delta') ? 'Thinking...' : (event as any).delta ?? 'Thinking...';

    case eventMatches(event, 'workflow.node.output.summary', 'workflow.node.output.summary'):
      if ('content' in event) {
        return (event as any).content;
      }
      return 'Response received';

    case eventMatches(event, 'workflow.tool.started', 'workflow.tool.started'):
      if ('tool_name' in event) {
        const preview =
          'arguments_preview' in event && event.arguments_preview
            ? event.arguments_preview
            : formatArgs((event as any).arguments);
        return preview ? `▸ ${event.tool_name}(${preview})` : `▸ ${event.tool_name}`;
      }
      return 'Tool executing';

    case eventMatches(event, 'workflow.tool.completed', 'workflow.tool.completed'):
      if ('tool_name' in event) {
        const icon = (event as any).error ? '✗' : '✓';
        const content = (event as any).error || formatResult((event as any).result);
        return `${icon} ${event.tool_name} → ${content}`;
      }
      return 'Tool complete';

    case eventMatches(event, 'workflow.node.completed', 'workflow.node.completed') &&
      typeof (event as any).iteration === 'number':
      return `✓ Iteration ${(event as any).iteration} complete (${(event as any).tokens_used} tokens, ${(event as any).tools_run} tools)`;

    case eventMatches(event, 'workflow.result.final', 'workflow.result.final'):
      if ('final_answer' in event) {
        const preview = (event as any).final_answer.slice(0, 150);
        const suffix = (event as any).final_answer.length > 150 ? '...' : '';
        return `✓ Task Complete\n${preview}${suffix}`;
      }
      return '✓ Task complete';

    case eventMatches(event, 'workflow.result.cancelled', 'workflow.result.cancelled'): {
      const requestedBy = 'requested_by' in event ? (event as any).requested_by : undefined;
      const actorPrefix = requestedBy === 'user' ? '⏹ You stopped the agent' : '⏹ Task cancelled';
      const reason =
        'reason' in event && (event as any).reason && (event as any).reason !== 'cancelled'
          ? ` · Reason: ${(event as any).reason}`
          : '';
      return `${actorPrefix}${reason}`;
    }

    case eventMatches(event, 'workflow.node.failed'):
      if ('error' in event) {
        return `✗ Error: ${(event as any).error}`;
      }
      return '✗ Error occurred';

    case eventMatches(event, 'workflow.node.started') &&
      typeof (event as any).step_index === 'number':
      return `→ Step ${(event as any).step_index + 1}: ${(event as any).step_description ?? ''}`;

    case eventMatches(event, 'workflow.node.completed') &&
      typeof (event as any).step_index === 'number':
      if ('step_result' in event && typeof (event as any).step_result === 'string') {
        const preview = (event as any).step_result ? (event as any).step_result.slice(0, 80) : '';
        return `✓ Step ${(event as any).step_index + 1} complete: ${preview}`;
      }
      return 'Step completed';

    case eventMatches(event, 'workflow.subflow.progress', 'workflow.subflow.progress'):
      return `↺ Subagent progress ${(event as any).completed}/${(event as any).total} · ${(event as any).tokens} tokens · ${(event as any).tool_calls} tool calls`;

    case eventMatches(event, 'workflow.subflow.completed', 'workflow.subflow.completed'):
      return `✓ Subagent summary ${(event as any).success}/${(event as any).total} succeeded (${(event as any).failed} failed, ${(event as any).tokens} tokens, ${(event as any).tool_calls} tool calls)`;

    case eventMatches(event, 'workflow.tool.progress', 'workflow.tool.progress'):
      if ('chunk' in event) {
        return (event as any).chunk;
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
