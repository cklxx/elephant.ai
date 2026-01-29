import React, { useState } from "react";
import { cn } from "@/lib/utils";
import { AgentCardProps } from "./types";
import { CardHeader } from "./CardHeader";
import { CardStats } from "./CardStats";
import { CardBody } from "./CardBody";
import { CardFooter } from "./CardFooter";
import { getStateColor } from "./styles";

export * from "./types";

export function AgentCard({
  data,
  expanded: controlledExpanded,
  onToggleExpand,
  resolvePairedToolStart,
  className,
}: AgentCardProps) {
  const [internalExpanded, setInternalExpanded] = useState(false);

  const isControlled = controlledExpanded !== undefined;
  const expanded = isControlled ? controlledExpanded : internalExpanded;
  const showInlineTokens = data.state === "completed" && data.stats.tokens > 0;

  const handleToggle = () => {
    if (onToggleExpand) {
      onToggleExpand();
    } else {
      setInternalExpanded(!internalExpanded);
    }
  };

  return (
    <div
      className={cn(
        "group my-2 -mx-2 px-2 transition-colors rounded-lg",
        "hover:bg-muted/5",
        className,
      )}
      data-testid="subagent-thread"
      data-subagent-key={data.id}
      data-agent-id={data.id}
      data-agent-state={data.state}
    >
      <div
        className={cn(
          "rounded-lg border-l-4 border-y border-r",
          "border-y-border/40 border-r-border/40",
          "bg-muted/10 transition-all duration-200",
          "group-hover:bg-muted/20 group-hover:shadow-sm",
          "overflow-hidden",
          getStateColor(data.state),
        )}
      >
        <div className="p-3 min-w-0 relative">
          <CardHeader
            state={data.state}
            preview={data.preview}
            type={data.type}
            concurrency={data.concurrency}
            inlineTokens={showInlineTokens ? data.stats.tokens : undefined}
          />

          <CardStats
            progress={data.progress}
            stats={data.stats}
            concurrency={
              data.concurrency && data.concurrency.total > 1
                ? `Parallel ×${data.concurrency.total}`
                : undefined
            }
            hideTokens={showInlineTokens}
          />

          <div className={cn("relative", expanded && "max-h-[300px] overflow-y-auto")}>
            <CardBody
              events={data.events}
              expanded={expanded}
              resolvePairedToolStart={resolvePairedToolStart}
            />
          </div>
          <CardFooter
            expanded={expanded}
            onToggle={handleToggle}
            eventCount={data.events.length}
          />
        </div>
      </div>
    </div>
  );
}
