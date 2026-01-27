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

  return (
    <div className="space-y-2 min-w-0">
      {hasProgress && (
        <div className="space-y-1 min-w-0">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">
              Progress: {progress.current}/{progress.total}
            </span>
            <span className="text-muted-foreground font-medium">
              {Math.round(progress.percentage)}%
            </span>
          </div>
          <div className="h-1.5 bg-muted rounded-full overflow-hidden">
            <div
              className={cn(
                "h-full transition-all duration-300 ease-out",
                progress.percentage >= 100
                  ? "bg-green-500 dark:bg-green-400"
                  : "bg-blue-500 dark:bg-blue-400",
              )}
              style={{ width: `${Math.min(progress.percentage, 100)}%` }}
            />
          </div>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3 text-[11px] text-muted-foreground">
        {concurrency && (
          <div className="flex items-center gap-1">
            <span>⚡</span>
            <span>{concurrency}</span>
          </div>
        )}
        {stats.toolCalls > 0 && (
          <div className="flex items-center gap-1">
            <span>🔧</span>
            <span>
              {stats.toolCalls} tool call{stats.toolCalls === 1 ? "" : "s"}
            </span>
          </div>
        )}
        {!hideTokens && stats.tokens > 0 && (
          <div className="flex items-center gap-1">
            <span>💬</span>
            <span>{formatTokens(stats.tokens)} tokens</span>
          </div>
        )}
        {stats.duration && (
          <div className="flex items-center gap-1">
            <span>⏱️</span>
            <span>{formatDuration(stats.duration)}</span>
          </div>
        )}
      </div>
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
