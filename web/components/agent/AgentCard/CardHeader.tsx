import React from "react";
import { Badge } from "@/components/ui/badge";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";
import { AgentCardState } from "./types";
import {
  getStateLabel,
  getStateIconContainerStyle,
  getStateIconColor,
  getStateLucideIcon,
} from "./styles";

interface CardHeaderProps {
  state: AgentCardState;
  preview?: string;
  type?: string;
  concurrency?: { index: number; total: number };
  inlineTokens?: number;
  hasEvents?: boolean;
  expanded?: boolean;
  onToggle?: () => void;
}

export function CardHeader({
  state,
  preview,
  type,
  concurrency,
  inlineTokens,
  hasEvents,
  expanded,
  onToggle,
}: CardHeaderProps) {
  const displayTitle = preview || type || "Sub Agent";
  const clickable = hasEvents && onToggle;
  const StateIcon = getStateLucideIcon(state);

  const content = (
    <div className="space-y-1 w-full">
      {/* Row 1: icon + title + chevron */}
      <div className="flex items-center gap-1.5 w-full min-w-0">
        <span
          className={cn(
            "inline-flex items-center justify-center h-4 w-4 rounded shrink-0",
            getStateIconContainerStyle(state),
          )}
        >
          <StateIcon
            className={cn(
              "h-3 w-3",
              getStateIconColor(state),
              state === "running" && "animate-spin",
            )}
          />
        </span>
        <span className="text-[13px] leading-snug text-foreground/80 truncate font-medium flex-1 min-w-0">
          {displayTitle}
        </span>
        {clickable && (
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 text-muted-foreground/50 shrink-0 transition-transform",
              expanded && "rotate-180",
            )}
          />
        )}
      </div>

      {/* Row 2: meta line */}
      <div className="pl-[22px] flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground leading-tight">
        <span className={getStateIconColor(state)}>{getStateLabel(state)}</span>
        {inlineTokens && inlineTokens > 0 && (
          <>
            <span aria-hidden="true">&middot;</span>
            <span data-testid="subagent-inline-tokens">
              {formatTokens(inlineTokens)} tokens
            </span>
          </>
        )}
        {concurrency && concurrency.total > 1 && (
          <>
            <span aria-hidden="true">&middot;</span>
            <Badge variant="outline" className="text-[10px] px-1.5 py-0">
              {concurrency.index}/{concurrency.total}
            </Badge>
          </>
        )}
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
