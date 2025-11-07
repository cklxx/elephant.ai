// Event styling configuration
// Maps event types to the research console design system color classes

import { AnyAgentEvent } from "@/lib/types";

/**
 * Event type to color class mapping
 * Uses the research console design system color palette
 */
interface EventStyle {
  content: string;
  line?: string;
}

const EVENT_STYLE_MAP: Record<string, EventStyle> = {
  user_task: {
    content: "font-semibold text-foreground",
    line: "is-highlighted",
  },
  task_analysis: {
    content: "text-muted-foreground text-sm",
    line: "is-task-analysis pr-0",
  },
  task_complete: {
    content: "font-semibold text-foreground",
    line: "is-highlighted",
  },
  error: { content: "font-semibold text-destructive", line: "is-highlighted" },
  research_plan: { content: "text-foreground/90 font-medium" },
  tool_call_start: { content: "font-mono text-[12px] text-foreground/80" },
  tool_call_complete: { content: "font-mono text-[12px] text-foreground/80" },
  thinking: { content: "text-muted-foreground italic" },
  think_complete: { content: "text-foreground" },
  step_started: {
    content: "text-foreground font-semibold uppercase tracking-[0.18em]",
  },
  step_completed: {
    content: "text-foreground font-semibold uppercase tracking-[0.18em]",
  },
  iteration_start: {
    content: "text-muted-foreground uppercase tracking-[0.2em]",
  },
  iteration_complete: {
    content: "text-foreground font-semibold uppercase tracking-[0.2em]",
  },
  tool_call_stream: { content: "text-muted-foreground font-mono text-[12px]" },
  browser_info: { content: "text-foreground/80" },
};

/**
 * Get Tailwind CSS classes for event styling
 * Returns appropriate color classes based on event type
 */
export function getEventStyle(event: AnyAgentEvent): EventStyle {
  return (
    EVENT_STYLE_MAP[event.event_type] || { content: "text-muted-foreground" }
  );
}
