// Event styling configuration
// Maps event types to Manus design system color classes

import { AnyAgentEvent } from '@/lib/types';

/**
 * Event type to color class mapping
 * Uses Manus design system color palette
 */
const EVENT_STYLE_MAP: Record<string, string> = {
  user_task: 'text-blue-700 font-semibold',
  task_analysis: 'text-blue-600',
  task_complete: 'text-green-500 font-semibold',
  error: 'text-red-500 font-semibold',
  research_plan: 'text-blue-600',
  tool_call_start: 'text-cyan-600',
  tool_call_complete: 'text-cyan-500',
  thinking: 'text-purple-600',
  think_complete: 'text-purple-500',
  step_started: 'text-yellow-600',
  step_completed: 'text-yellow-500',
  iteration_start: 'text-gray-600',
  iteration_complete: 'text-gray-500',
  tool_call_stream: 'text-cyan-400',
  browser_snapshot: 'text-indigo-600',
};

/**
 * Get Tailwind CSS classes for event styling
 * Returns appropriate color classes based on event type
 */
export function getEventStyle(event: AnyAgentEvent): string {
  return EVENT_STYLE_MAP[event.event_type] || 'text-muted-foreground';
}
