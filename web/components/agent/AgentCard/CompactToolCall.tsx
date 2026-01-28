import React, { useState } from "react";
import { cn } from "@/lib/utils";

interface CompactToolCallProps {
  toolName: string;
  success: boolean;
  result?: string;
  error?: string;
  duration?: number;
  parameters?: Record<string, unknown>;
}

export function CompactToolCall({
  toolName,
  success,
  result,
  error,
  duration,
  parameters,
}: CompactToolCallProps) {
  const [expanded, setExpanded] = useState(false);
  const normalizedToolName = toolName || "unknown";
  const testId = `compact-tool-call-${normalizedToolName
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")}`;

  const hasDetails = parameters && Object.keys(parameters).length > 0;
  const displayResult = error || result || "No output";
  const shortResult = displayResult.length > 80 ? displayResult.slice(0, 80) + "..." : displayResult;

  return (
    <div className="text-xs space-y-1" data-testid={testId}>
      <div
        className={cn(
          "flex items-start gap-2 p-2 rounded-md transition-colors",
          success ? "bg-green-50 dark:bg-green-950/20" : "bg-red-50 dark:bg-red-950/20",
          hasDetails && "cursor-pointer hover:opacity-80",
        )}
        onClick={() => hasDetails && setExpanded(!expanded)}
      >
        <span className="text-base leading-none shrink-0">
          {success ? "✓" : "✗"}
        </span>
        <div className="flex-1 min-w-0 space-y-0.5">
          <div className="flex items-baseline gap-2 flex-wrap">
            <span className={cn(
              "font-medium",
              success ? "text-green-700 dark:text-green-400" : "text-red-700 dark:text-red-400"
            )}>
              {normalizedToolName}
            </span>
            {duration !== undefined && (
              <span className="text-[10px] text-muted-foreground">
                {formatDuration(duration)}
              </span>
            )}
          </div>
          <div className={cn(
            "text-[11px] break-words",
            success ? "text-green-600 dark:text-green-500/80" : "text-red-600 dark:text-red-500/80"
          )}>
            {shortResult}
          </div>
        </div>
        {hasDetails && (
          <span className={cn(
            "text-[10px] text-muted-foreground transition-transform shrink-0",
            expanded && "rotate-180"
          )}>
            ▼
          </span>
        )}
      </div>

      {expanded && hasDetails && (
        <div className="ml-6 pl-3 border-l-2 border-border/30 space-y-1 text-[11px]">
          <div className="text-muted-foreground font-medium">Parameters:</div>
          <div className="space-y-0.5">
            {Object.entries(parameters).map(([key, value]) => (
              <div key={key} className="break-words">
                <span className="text-muted-foreground">{key}:</span>{" "}
                <span className="text-foreground/80">
                  {formatValue(value)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`;
  }
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) {
    return String(value);
  }
  if (typeof value === "string") {
    return value.length > 100 ? value.slice(0, 100) + "..." : value;
  }
  if (typeof value === "object") {
    const str = JSON.stringify(value);
    return str.length > 100 ? str.slice(0, 100) + "..." : str;
  }
  return String(value);
}
