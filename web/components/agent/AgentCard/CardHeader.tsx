import React from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { AgentCardState } from "./types";
import { getStateIcon, getStateLabel, getStateBadgeColor } from "./styles";

interface CardHeaderProps {
  state: AgentCardState;
  preview?: string;
  type?: string;
  concurrency?: { index: number; total: number };
}

export function CardHeader({
  state,
  preview,
  type,
  concurrency,
}: CardHeaderProps) {
  return (
    <div className="flex items-start justify-between gap-2">
      <div className="flex items-center gap-2 flex-1 min-w-0">
        <span className="text-lg leading-none" role="img" aria-label={state}>
          {getStateIcon(state)}
        </span>
        {preview && (
          <span className="text-sm text-foreground/80 truncate font-medium">
            {preview}
          </span>
        )}
        {type && !preview && (
          <span className="text-sm text-foreground/80 font-medium">{type}</span>
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
          )}
        >
          {getStateLabel(state)}
        </Badge>
      </div>
    </div>
  );
}
