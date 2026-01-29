import React, { useState } from "react";
import { cn } from "@/lib/utils";
import { AgentCardProps } from "./types";
import { CardHeader } from "./CardHeader";
import { CardStats } from "./CardStats";
import { CardBody } from "./CardBody";
import { CardFooter } from "./CardFooter";
import { getStateAccentColor, getStateContainerStyle } from "./styles";

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
      className={cn("my-2", className)}
      data-testid="subagent-thread"
      data-subagent-key={data.id}
      data-agent-id={data.id}
      data-agent-state={data.state}
    >
      <div
        className={cn(
          "rounded-lg border border-border/40",
          "transition-colors duration-200",
          "overflow-hidden",
          getStateContainerStyle(data.state),
        )}
      >
        {data.state !== "idle" && (
          <div className={cn("h-0.5 w-full", getStateAccentColor(data.state))} />
        )}
        <div className="px-3 py-2.5 min-w-0 space-y-2">
          <CardHeader
            state={data.state}
            preview={data.preview}
            type={data.type}
            concurrency={data.concurrency}
            inlineTokens={showInlineTokens ? data.stats.tokens : undefined}
            hasEvents={data.events.length > 0}
            expanded={expanded}
            onToggle={handleToggle}
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
