import React from "react";
import { cn } from "@/lib/utils";
import { AgentCardProgress, AgentCardStats } from "./types";

interface CardStatsProps {
  progress?: AgentCardProgress;
  stats: AgentCardStats;
  concurrency?: string;
  hideTokens?: boolean;
}

export function CardStats({ progress, stats, concurrency, hideTokens }: CardStatsProps) {
  const hasProgress = progress && progress.total > 0;
  const hasStatsRow =
    Boolean(concurrency) ||
    stats.toolCalls > 0 ||
    (!hideTokens && stats.tokens > 0) ||
    Boolean(stats.duration);

  if (!hasProgress && !hasStatsRow) {
    return null;
  }

  return (
    <div className="pl-6 space-y-1 min-w-0">
      {hasProgress && (
        <div className="flex items-center gap-2 min-w-0">
          <div className="h-1 flex-1 bg-muted rounded-full overflow-hidden">
            <div
              className={cn(
                "h-full transition-all duration-300 ease-out",
                progress.percentage >= 100
                  ? "bg-emerald-500/70"
                  : "bg-blue-500/70",
              )}
              style={{ width: `${Math.min(progress.percentage, 100)}%` }}
            />
          </div>
          <span className="text-[10px] text-muted-foreground shrink-0">
            {progress.current}/{progress.total}
          </span>
        </div>
      )}

      <StatsRow concurrency={concurrency} stats={stats} hideTokens={hideTokens} />
    </div>
  );
}

function StatsRow({
  concurrency,
  stats,
  hideTokens,
}: {
  concurrency?: string;
  stats: AgentCardStats;
  hideTokens?: boolean;
}) {
  const items: string[] = [];
  if (concurrency) items.push(concurrency);
  if (stats.toolCalls > 0)
    items.push(`${stats.toolCalls} tool call${stats.toolCalls === 1 ? "" : "s"}`);
  if (!hideTokens && stats.tokens > 0)
    items.push(`${formatTokens(stats.tokens)} tokens`);
  if (stats.duration) items.push(formatDuration(stats.duration));

  if (items.length === 0) return null;

  return (
    <div className="flex flex-wrap items-center gap-1.5 text-[10px] text-muted-foreground leading-tight">
      {items.map((item, i) => (
        <React.Fragment key={i}>
          {i > 0 && <span aria-hidden="true">&middot;</span>}
          <span>{item}</span>
        </React.Fragment>
      ))}
    </div>
  );
}

function formatTokens(tokens: number): string {
  if (tokens >= 1000) {
    return `${(tokens / 1000).toFixed(1)}K`;
  }
  return tokens.toString();
}

function formatDuration(ms: number): string {
  const seconds = ms / 1000;
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`;
  }
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = Math.floor(seconds % 60);
  return `${minutes}m ${remainingSeconds}s`;
}
