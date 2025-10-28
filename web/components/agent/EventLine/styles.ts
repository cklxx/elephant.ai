// Event styling configuration
// Maps event types to the research console design system color classes

import { AnyAgentEvent } from '@/lib/types';

/**
 * Event type to color class mapping
 * Uses the research console design system color palette
 */
const EVENT_STYLE_MAP: Record<string, string> = {
  user_task: 'text-slate-700 font-semibold',
  task_analysis: 'text-slate-700',
  task_complete: 'text-emerald-700 font-semibold',
  error: 'text-red-600 font-semibold',
  research_plan: 'text-slate-700',
  tool_call_start: 'text-slate-700',
  tool_call_complete: 'text-slate-700',
  thinking: 'text-slate-500 italic',
  think_complete: 'text-slate-900',
  step_started: 'text-slate-700',
  step_completed: 'text-emerald-700 font-medium',
  iteration_start: 'text-slate-500',
  iteration_complete: 'text-slate-500',
  tool_call_stream: 'text-slate-700',
  browser_snapshot: 'text-slate-700',
};

/**
 * Get Tailwind CSS classes for event styling
 * Returns appropriate color classes based on event type
 */
export function getEventStyle(event: AnyAgentEvent): string {
  return EVENT_STYLE_MAP[event.event_type] || 'text-muted-foreground';
}
