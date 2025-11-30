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
  "workflow.input.received": {
    content: "font-semibold text-foreground",
    line: "is-highlighted",
  },
  "workflow.result.final": {
    content: "font-semibold text-foreground",
    line: "is-highlighted",
  },
  "workflow.result.cancelled": {
    content: "font-semibold text-amber-600",
    line: "is-highlighted",
  },
  "workflow.node.failed": { content: "font-semibold text-destructive", line: "is-highlighted" },
  "workflow.tool.started": { content: "font-mono text-[12px] text-foreground/80" },
  "workflow.tool.completed": { content: "font-mono text-[12px] text-foreground/80" },
  "workflow.node.output.delta": { content: "text-muted-foreground italic" },
  "workflow.node.output.summary": { content: "text-foreground" },
  "workflow.node.started": {
    content: "text-muted-foreground uppercase tracking-[0.2em]",
  },
  "workflow.node.completed": {
    content: "text-foreground font-semibold uppercase tracking-[0.2em]",
  },
  "workflow.tool.progress": { content: "text-muted-foreground font-mono text-[12px]" },
  "workflow.diagnostic.browser_info": { content: "text-foreground/80" },
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
