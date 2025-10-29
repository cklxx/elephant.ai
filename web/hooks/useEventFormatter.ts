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
 *     user_task: (event) => `Custom: ${event.task}`
 *   }
 * });
 * ```
 */

import { useMemo } from 'react';
import { AnyAgentEvent } from '@/lib/types';

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
      switch (eventType) {
        case 'user_task':
          return 'text-primary font-semibold';
        case 'task_analysis':
          return 'text-primary';
        case 'task_complete':
          return 'text-emerald-600 font-semibold';
        case 'error':
          return 'text-destructive font-semibold';
        case 'research_plan':
          return 'text-primary';
        case 'tool_call_start':
          return 'text-primary';
        case 'tool_call_complete':
          return 'text-primary';
        case 'thinking':
          return 'text-muted-foreground';
        case 'think_complete':
          return 'text-muted-foreground';
        case 'step_started':
          return 'text-primary';
        case 'step_completed':
          return 'text-emerald-600 font-medium';
        case 'iteration_start':
          return 'text-muted-foreground';
        case 'iteration_complete':
          return 'text-muted-foreground';
        default:
          return 'text-muted-foreground';
      }
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
            const preview = event.content.slice(0, maxContentLength);
            const suffix = event.content.length > maxContentLength ? '...' : '';
            return `✓ Response: ${preview}${suffix}`;
          }
          return '✓ Response received';

        case 'tool_call_start':
          if ('tool_name' in event) {
            const preview =
              'arguments_preview' in event && event.arguments_preview
                ? event.arguments_preview
                : formatArgs(event.arguments);
            return preview
              ? `▸ ${event.tool_name}(${preview})`
              : `▸ ${event.tool_name}`;
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
            const preview = event.final_answer.slice(0, maxContentLength + 50);
            const suffix = event.final_answer.length > maxContentLength + 50 ? '...' : '';
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

        case 'browser_info':
          if ('message' in event && event.message) {
            return `🧭 Browser diagnostics: ${event.message}`;
          }
          return '🧭 Browser diagnostics captured';

        case 'tool_call_stream':
          if ('chunk' in event) {
            return event.chunk;
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
