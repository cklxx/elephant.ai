import React from "react";
import { AnyAgentEvent, WorkflowToolStartedEvent } from "@/lib/types";
import { EventLine } from "../EventLine";
import { cn } from "@/lib/utils";

interface CardBodyProps {
  events: AnyAgentEvent[];
  expanded: boolean;
  resolvePairedToolStart?: (event: AnyAgentEvent) => WorkflowToolStartedEvent | undefined;
}

export function CardBody({
  events,
  expanded,
  resolvePairedToolStart,
}: CardBodyProps) {
  if (events.length === 0) {
    return null;
  }

  const displayEvents = expanded ? events : events.slice(-1);

  return (
    <div className="space-y-1 min-w-0 w-full overflow-x-hidden">
      {displayEvents.map((event, i) => {
        const pairedToolStart = resolvePairedToolStart
          ? resolvePairedToolStart(event)
          : undefined;
        const actualIndex = expanded ? i : events.length - 1;

        return (
          <div
            key={`event-${actualIndex}-${event.event_type}-${event.timestamp}`}
            className={cn(
              "transition-colors rounded-md hover:bg-muted/10 -mx-2 px-2",
              "min-w-0 w-full overflow-x-hidden break-words text-muted-foreground/70",
              !expanded && "max-h-[3.75rem] overflow-hidden",
            )}
          >
            <EventLine
              event={event}
              showSubagentContext={false}
              pairedToolStartEvent={pairedToolStart}
              variant="nested"
            />
          </div>
        );
      })}
    </div>
  );
}
