// Event styling configuration
// Maps event types to the research console design system color classes

import { AnyAgentEvent } from '@/lib/types';

/**
 * Event type to color class mapping
 * Uses the research console design system color palette
 */
const EVENT_STYLE_MAP: Record<string, string> = {
  user_task: 'text-primary font-semibold',
  task_analysis: 'text-primary',
  task_complete: 'text-emerald-600 font-semibold',
  error: 'text-destructive font-semibold',
  research_plan: 'text-primary',
  tool_call_start: 'text-primary',
  tool_call_complete: 'text-primary',
  thinking: 'text-muted-foreground',
  think_complete: 'text-muted-foreground',
  step_started: 'text-primary',
  step_completed: 'text-emerald-600 font-medium',
  iteration_start: 'text-muted-foreground',
  iteration_complete: 'text-muted-foreground',
  tool_call_stream: 'text-primary',
  browser_snapshot: 'text-primary',
};

/**
 * Get Tailwind CSS classes for event styling
 * Returns appropriate color classes based on event type
 */
export function getEventStyle(event: AnyAgentEvent): string {
  return EVENT_STYLE_MAP[event.event_type] || 'text-muted-foreground';
}
