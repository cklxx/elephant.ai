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
  if (!expanded || events.length === 0) {
    return null;
  }

  return (
    <div className="space-y-1 max-h-[400px] overflow-y-auto">
      {events.map((event, i) => {
        const pairedToolStart = resolvePairedToolStart ? resolvePairedToolStart(event) : undefined;

        return (
          <div
            key={`event-${i}-${event.event_type}-${event.timestamp}`}
            className={cn(
              "transition-colors rounded-md hover:bg-muted/10 -mx-2 px-2",
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
