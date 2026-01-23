/**
 * Event Formatter Hook
 *
 * Provides memoized formatting functions for agent events with:
 * - Event styling based on type
 * - Content formatting with truncation
 * - Customizable format overrides
 * - Performance optimization via memoization
 *
 * @example
 * ```tsx
 * const { formatContent, getEventStyle } = useEventFormatter({
 *   maxContentLength: 150,
 *   formatOverrides: {
 *     workflow.input.received: (event) => `Custom: ${event.task}`
 *   }
 * });
 * ```
 */

import { useMemo } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { isEventType } from '@/lib/events/matching';

/**
 * Custom formatter function type
 */
type EventFormatter = (event: AnyAgentEvent) => string;

/**
 * Configuration options for event formatting
 */
interface UseEventFormatterOptions {
  /** Maximum length for truncated content (default: 100) */
  maxContentLength?: number;
  /** Custom format functions for specific event types */
  formatOverrides?: Partial<Record<AnyAgentEvent['event_type'], EventFormatter>>;
}

/**
 * Return value of useEventFormatter hook
 */
interface UseEventFormatterReturn {
  /** Get CSS class name for event styling */
  getEventStyle: (eventType: AnyAgentEvent['event_type']) => string;
  /** Format event content for display */
  formatContent: (event: AnyAgentEvent) => string;
  /** Format timestamp to HH:MM:SS */
  formatTimestamp: (timestamp: string | number) => string;
  /** Format tool arguments for display */
  formatArgs: (args: any) => string;
  /** Format tool result for display */
  formatResult: (result: any) => string;
}

/**
 * Hook for formatting agent events consistently
 */
export function useEventFormatter(
  options: UseEventFormatterOptions = {}
): UseEventFormatterReturn {
  const {
    maxContentLength = 100,
    formatOverrides = {},
  } = options;

  /**
   * Get CSS class name for event type styling
   * Memoized to prevent recalculation on every render
   */
  const getEventStyle = useMemo(
    () => (eventType: AnyAgentEvent['event_type']): string => {
      if (eventType === 'workflow.input.received') return 'text-primary font-semibold';
      if (['workflow.result.final', 'workflow.result.final'].includes(eventType)) {
        return 'text-emerald-600 font-semibold';
      }
      if (['workflow.result.cancelled', 'workflow.result.cancelled'].includes(eventType)) {
        return 'text-amber-600 font-semibold';
      }
      if (['workflow.node.failed'].includes(eventType)) {
        return 'text-destructive font-semibold';
      }
      if (['workflow.subflow.progress', 'workflow.subflow.progress'].includes(eventType)) {
        return 'text-muted-foreground';
      }
      if (['workflow.subflow.completed', 'workflow.subflow.completed'].includes(eventType)) {
        return 'text-emerald-600 font-semibold';
      }
      if (['workflow.tool.started', 'workflow.tool.started'].includes(eventType)) {
        return 'text-primary';
      }
      if (['workflow.tool.completed', 'workflow.tool.completed'].includes(eventType)) {
        return 'text-primary';
      }
      if (['workflow.node.output.delta', 'workflow.node.output.delta'].includes(eventType)) {
        return 'text-muted-foreground';
      }
      if (['workflow.node.output.summary', 'workflow.node.output.summary'].includes(eventType)) {
        return 'text-muted-foreground';
      }
      if (['workflow.node.started'].includes(eventType)) {
        return 'text-muted-foreground';
      }
      if (['workflow.node.completed'].includes(eventType)) {
        return 'text-emerald-600 font-medium';
      }
      return 'text-muted-foreground';
    },
    []
  );

  /**
   * Format timestamp to HH:MM:SS format
   */
  const formatTimestamp = useMemo(
    () => (timestamp: string | number): string => {
      return new Date(timestamp || Date.now()).toLocaleTimeString('en-US', {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      });
    },
    []
  );

  /**
   * Format tool arguments for display
   * Truncates and formats complex arguments
   */
  const formatArgs = useMemo(
    () => (args: any): string => {
      if (!args) return '';
      if (typeof args === 'string') return args;

      const keys = Object.keys(args);
      if (keys.length === 0) return '';
      if (keys.length === 1) return String(args[keys[0]]);

      return keys
        .slice(0, 2)
        .map((k) => `${k}: ${String(args[k]).slice(0, 20)}`)
        .join(', ');
    },
    []
  );

  /**
   * Format tool result for display
   * Handles various result formats and truncates long output
   */
  const formatResult = useMemo(
    () => (result: any): string => {
      if (!result) return '';
      if (typeof result === 'string') return result.slice(0, maxContentLength);
      if (result.output) return String(result.output).slice(0, maxContentLength);
      if (result.content) return String(result.content).slice(0, maxContentLength);
      return JSON.stringify(result).slice(0, maxContentLength);
    },
    [maxContentLength]
  );

  /**
   * Format event content for display
   * Uses custom overrides if provided, otherwise uses default formatting
   */
  const formatContent = useMemo(
    () => (event: AnyAgentEvent): string => {
      // Check for custom override first
      const override = formatOverrides[event.event_type];
      if (override) {
        return override(event);
      }

      // Default formatting logic
      switch (true) {
        case event.event_type === 'workflow.input.received':
          if ('task' in event) {
            return `👤 User: ${(event as any).task}`;
          }
          return 'User task';

        case isEventType(event, 'workflow.node.started') &&
          typeof (event as any).iteration === 'number':
          return `→ Iteration ${(event as any).iteration}/${(event as any).total_iters}`;

        case isEventType(event, 'workflow.node.output.delta'):
          if ('iteration' in event && typeof (event as any).iteration === 'number') {
            return `💭 Thinking... (iteration ${(event as any).iteration})`;
          }
          return '💭 Thinking...';

        case isEventType(event, 'workflow.node.output.summary'):
          if ('content' in event) {
            const preview = (event as any).content.slice(0, maxContentLength);
            const suffix = (event as any).content.length > maxContentLength ? '...' : '';
            return `✓ Response: ${preview}${suffix}`;
          }
          return '✓ Response received';

        case isEventType(event, 'workflow.tool.started'):
          if ('tool_name' in event) {
            const preview =
              'arguments_preview' in event && (event as any).arguments_preview
                ? (event as any).arguments_preview
                : formatArgs((event as any).arguments);
            return preview ? `▸ ${event.tool_name}(${preview})` : `▸ ${event.tool_name}`;
          }
          return 'Tool executing';

        case isEventType(event, 'workflow.tool.completed'):
          if ('tool_name' in event) {
            const icon = (event as any).error ? '✗' : '✓';
            const content = (event as any).error || formatResult((event as any).result);
            return `${icon} ${event.tool_name} → ${content}`;
          }
          return 'Tool complete';

        case isEventType(event, 'workflow.node.completed') &&
          typeof (event as any).iteration === 'number':
          return `✓ Iteration ${(event as any).iteration} complete (${(event as any).tokens_used} tokens, ${(event as any).tools_run} tools)`;

        case isEventType(event, 'workflow.result.final'):
          if ('final_answer' in event) {
            const preview = (event as any).final_answer.slice(0, maxContentLength + 50);
            const suffix = (event as any).final_answer.length > maxContentLength + 50 ? '...' : '';
            return `✓ Task Complete\n${preview}${suffix}`;
          }
          return '✓ Task complete';

        case isEventType(event, 'workflow.result.cancelled'): {
          const requestedBy = 'requested_by' in event ? (event as any).requested_by : undefined;
          const prefix = requestedBy === 'user' ? '⏹ You stopped the agent' : '⏹ Task cancelled';
          const reason =
            'reason' in event && (event as any).reason && (event as any).reason !== 'cancelled'
              ? ` · Reason: ${(event as any).reason}`
              : '';
          return `${prefix}${reason}`;
        }

        case isEventType(event, 'workflow.node.failed'):
          if ('error' in event) {
            return `✗ Error: ${(event as any).error}`;
          }
          return '✗ Error occurred';

        case isEventType(event, 'workflow.node.started') &&
          typeof (event as any).step_index === 'number':
          return `→ Step ${(event as any).step_index + 1}: ${(event as any).step_description ?? ''}`;

        case isEventType(event, 'workflow.node.completed') &&
          typeof (event as any).step_index === 'number':
          if ('step_result' in event && typeof (event as any).step_result === 'string') {
            const preview = (event as any).step_result ? (event as any).step_result.slice(0, 80) : '';
            return `✓ Step ${(event as any).step_index + 1} complete: ${preview}`;
          }
          return 'Step completed';

        case isEventType(event, 'workflow.subflow.progress'):
          return `↺ Subagent progress ${(event as any).completed}/${(event as any).total} · ${(event as any).tokens} tokens · ${(event as any).tool_calls} tool calls`;

        case isEventType(event, 'workflow.subflow.completed'):
          return `✓ Subagent summary ${(event as any).success}/${(event as any).total} succeeded (${(event as any).failed} failed, ${(event as any).tokens} tokens, ${(event as any).tool_calls} tool calls)`;

        case isEventType(event, 'workflow.tool.progress'):
          if ('chunk' in event) {
            return (event as any).chunk;
          }
          return '';

        default:
          return JSON.stringify(event).slice(0, maxContentLength);
      }
    },
    [maxContentLength, formatOverrides, formatArgs, formatResult]
  );

  return {
    getEventStyle,
    formatContent,
    formatTimestamp,
    formatArgs,
    formatResult,
  };
}
