import React, { useState } from "react";
import { Check, X, ChevronDown } from "lucide-react";
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
          "flex items-start gap-2 p-2 rounded-md border border-border/30",
          hasDetails && "cursor-pointer hover:bg-muted/10",
        )}
        onClick={() => hasDetails && setExpanded(!expanded)}
      >
        {success ? (
          <Check className="h-3 w-3 text-emerald-600 dark:text-emerald-400 shrink-0 mt-0.5" />
        ) : (
          <X className="h-3 w-3 text-red-600 dark:text-red-400 shrink-0 mt-0.5" />
        )}
        <div className="flex-1 min-w-0 space-y-0.5">
          <div className="flex items-baseline gap-2 flex-wrap">
            <span className="font-medium text-foreground/80">
              {normalizedToolName}
            </span>
            {duration !== undefined && (
              <span className="text-[10px] text-muted-foreground">
                {formatDuration(duration)}
              </span>
            )}
          </div>
          <div className="text-[11px] break-words text-muted-foreground/70">
            {shortResult}
          </div>
        </div>
        {hasDetails && (
          <ChevronDown
            className={cn(
              "h-3 w-3 text-muted-foreground shrink-0 transition-transform",
              expanded && "rotate-180",
            )}
          />
        )}
      </div>

      {expanded && hasDetails && (
        <div className="ml-6 pl-3 border-l border-border/30 space-y-1 text-[11px]">
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
