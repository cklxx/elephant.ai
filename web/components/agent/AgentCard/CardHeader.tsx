import React from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { AgentCardState } from "./types";
import { getStateLabel, getStateIcon, getStateBadgeColor } from "./styles";

interface CardHeaderProps {
  state: AgentCardState;
  preview?: string;
  type?: string;
  concurrency?: { index: number; total: number };
  inlineTokens?: number;
  hasEvents?: boolean;
  onToggle?: () => void;
}

export function CardHeader({
  state,
  preview,
  type,
  concurrency,
  inlineTokens,
  hasEvents,
  onToggle,
}: CardHeaderProps) {
  const displayTitle = preview || type || "Sub Agent";
  const clickable = hasEvents && onToggle;

  const content = (
    <div className="flex items-start justify-between gap-2 w-full">
      <div className="flex items-center gap-2 flex-1 min-w-0">
        <span className="text-sm text-foreground/80 truncate font-medium min-w-0">
          {displayTitle}
        </span>
        {inlineTokens && inlineTokens > 0 && (
          <span
            className="text-[11px] text-muted-foreground shrink-0"
            data-testid="subagent-inline-tokens"
          >
            {formatTokens(inlineTokens)} tokens
          </span>
        )}
      </div>
      <div className="flex items-center gap-2 shrink-0">
        {concurrency && concurrency.total > 1 && (
          <Badge variant="outline" className="text-[10px] px-1.5 py-0">
            {concurrency.index}/{concurrency.total}
          </Badge>
        )}
        <Badge
          className={cn(
            "text-[10px] px-1.5 py-0.5 font-medium",
            getStateBadgeColor(state),
            state === "running" && "animate-pulse",
          )}
        >
          {getStateIcon(state)} {getStateLabel(state)}
        </Badge>
      </div>
    </div>
  );

  if (clickable) {
    return (
      <button
        type="button"
        onClick={onToggle}
        className="w-full text-left cursor-pointer focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring rounded-sm"
        aria-label={`Toggle ${displayTitle} details`}
      >
        {content}
      </button>
    );
  }

  return content;
}

function formatTokens(tokens: number): string {
  if (tokens >= 1000) {
    return `${(tokens / 1000).toFixed(1)}K`;
  }
  return tokens.toString();
}
