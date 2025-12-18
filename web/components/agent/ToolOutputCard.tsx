"use client";

import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { formatDuration, cn, humanizeToolName, getToolIcon } from "@/lib/utils";
import { ChevronRight, Loader2 } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { AttachmentPayload } from "@/lib/types";
import { isDebugModeEnabled } from "@/lib/debugMode";
import { userFacingToolSummary } from "@/lib/toolPresentation";
import { useElapsedDurationMs } from "@/hooks/useElapsedDurationMs";
import {
  ToolArgumentsPanel,
  ToolResultPanel,
  ToolStreamPanel,
} from "./tooling/ToolPanels";

interface ToolOutputCardProps {
  toolName: string;
  parameters?: Record<string, unknown>;
  result?: string;
  error?: string;
  duration?: number;
  timestamp?: string;
  callId?: string;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
  status?: "running" | "completed" | "failed";
}

export function ToolOutputCard({
  toolName,
  parameters,
  result,
  error,
  duration,
  timestamp,
  callId,
  metadata,
  attachments,
  status,
}: ToolOutputCardProps) {
  const hasResult = Boolean(result && result.trim().length > 0);
  const hasParameters = Boolean(
    parameters && Object.keys(parameters).length > 0,
  );
  const hasError = Boolean(error && error.trim().length > 0);
  const [isExpanded, setIsExpanded] = useState(false); // Default collapsed for Manus style
  const t = useTranslation();
  const toolIcon = useMemo(() => getToolIcon(toolName), [toolName]);

  const normalizedToolName = toolName.toLowerCase();

  // Humanize tool Name
  const displayToolName = useMemo(() => {
    return humanizeToolName(toolName);
  }, [toolName]);

  const resolvedStatus: "running" | "completed" | "failed" = useMemo(() => {
    if (status) return status;
    if (hasError) return "failed";
    return "completed";
  }, [status, hasError]);

  const statusLabel = useMemo(() => {
    switch (resolvedStatus) {
      case "running":
        return t("tool.status.running");
      case "failed":
        return t("tool.status.failed");
      default:
        return t("tool.status.completed");
    }
  }, [resolvedStatus, t]);

  const debugMode = useMemo(() => isDebugModeEnabled(), []);

  const elapsedMs = useElapsedDurationMs({
    startTimestamp: timestamp ?? null,
    running: resolvedStatus === "running",
    tickMs: 250,
  });

  const displayDurationMs = useMemo(() => {
    if (resolvedStatus === "running") {
      return typeof elapsedMs === "number" ? elapsedMs : null;
    }
    return typeof duration === "number" && duration > 0 ? duration : null;
  }, [duration, elapsedMs, resolvedStatus]);

  const previewText = useMemo(() => {
    if (error) return error;
    if (result) {
      const summary = userFacingToolSummary({
        toolName,
        result,
        error: null,
        metadata: (metadata as Record<string, any>) ?? null,
        attachments: (attachments as any) ?? null,
      });
      if (summary) {
        return summary;
      }
      const trimmed = result.trim();
      return trimmed.length > 100 ? trimmed.slice(0, 100) + "..." : trimmed;
    }
    // Fallback to params
    return formatParams(parameters, toolName) || "";
  }, [error, result, parameters, toolName, metadata, attachments]);

  const attachmentCount = useMemo(
    () => (attachments ? Object.keys(attachments).length : 0),
    [attachments],
  );

  const hasMetadata =
    Boolean(metadata) &&
    typeof metadata === "object" &&
    Object.keys(metadata ?? {}).length > 0;
  const showBody =
    hasResult || hasParameters || hasError || hasMetadata || attachmentCount > 0;

  const formattedArguments = useMemo(() => {
    if (!parameters || Object.keys(parameters).length === 0) {
      return "";
    }
    try {
      return JSON.stringify(parameters, null, 2);
    } catch {
      return String(parameters);
    }
  }, [parameters]);

  const formattedMetadata = useMemo(() => {
    if (!hasMetadata) {
      return "";
    }
    try {
      return JSON.stringify(metadata, null, 2);
    } catch {
      return String(metadata);
    }
  }, [hasMetadata, metadata]);

  const toggleLabel = isExpanded
    ? t("tool.toggle.collapse")
    : t("tool.toggle.expand");

  const statusBadgeVariant = useMemo(() => {
    switch (resolvedStatus) {
      case "running":
        return "info" as const;
      case "failed":
        return "destructive" as const;
      default:
        return "success" as const;
    }
  }, [resolvedStatus]);

  const timestampLabel = useMemo(() => {
    if (!timestamp) {
      return null;
    }
    const parsed = new Date(timestamp);
    if (Number.isNaN(parsed.getTime())) {
      return timestamp;
    }
    return parsed.toISOString().slice(11, 19);
  }, [timestamp]);

  return (
    <div
      className="group mb-2"
      data-testid={`tool-output-card-${normalizedToolName.replace(/\s+/g, '-')}`}
    >
      <button
        type="button"
        onClick={() => setIsExpanded((prev) => !prev)}
        aria-expanded={isExpanded}
        data-testid="tool-output-header"
        title={toggleLabel}
        className={cn(
          "flex w-full items-start gap-3 px-3 py-2 text-left",
          "text-[13px] leading-snug",
          "cursor-pointer select-none rounded-md border border-border/40",
          "bg-secondary/40 transition-colors hover:bg-secondary/60",
          resolvedStatus === "running" &&
            "bg-blue-50/50 border-blue-100/50 text-blue-900 dark:bg-blue-900/20 dark:text-blue-100 dark:border-blue-800/30",
          resolvedStatus === "failed" &&
            "bg-red-50/50 border-red-100/50 text-red-900 dark:bg-red-900/20 dark:text-red-100 dark:border-red-800/30",
        )}
      >
        <div
          className={cn(
            "relative mt-0.5 flex h-7 w-7 flex-none items-center justify-center rounded-md border border-border/60 bg-background/40",
            resolvedStatus === "running" &&
              "border-blue-200/60 bg-blue-50/40 dark:border-blue-800/30 dark:bg-blue-950/30",
            resolvedStatus === "failed" &&
              "border-red-200/60 bg-red-50/40 dark:border-red-800/30 dark:bg-red-950/30",
          )}
        >
          <span className="text-[13px] leading-none" aria-hidden="true">
            {toolIcon}
          </span>
        </div>

        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex items-start justify-between gap-3">
            <span
              className="min-w-0 flex-1 truncate text-[13px] font-semibold tracking-tight"
              data-testid="tool-name"
            >
              {displayToolName}
            </span>

            <div className="flex flex-none flex-wrap items-center justify-end gap-2">
              <Badge
                variant={statusBadgeVariant}
                className="rounded-md px-2 py-0.5 text-[10px]"
              >
                {resolvedStatus === "running" ? (
                  <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
                ) : null}
                {statusLabel}
              </Badge>
              {typeof displayDurationMs === "number" && displayDurationMs > 0 && (
                <Badge
                  variant="outline"
                  className="rounded-md px-2 py-0.5 text-[10px] font-mono tabular-nums text-muted-foreground"
                >
                  {formatDuration(displayDurationMs)}
                </Badge>
              )}
              {attachmentCount > 0 && (
                <Badge
                  variant="secondary"
                  className="rounded-md px-2 py-0.5 text-[10px] tabular-nums text-muted-foreground"
                >
                  {attachmentCount} attachment{attachmentCount === 1 ? "" : "s"}
                </Badge>
              )}
            </div>
          </div>

          {previewText ? (
            <p
              className="line-clamp-2 text-[12px] leading-snug text-muted-foreground/70"
              data-testid="tool-preview"
            >
              {previewText}
            </p>
          ) : null}

          {(timestamp || (debugMode && callId)) && (
            <p className="flex flex-wrap gap-x-3 gap-y-1 text-[10px] font-mono text-muted-foreground/60">
              {debugMode && callId ? (
                <span>
                  {t("events.toolCall.id")}: {callId}
                </span>
              ) : null}
              {timestampLabel ? (
                <span>{timestampLabel}</span>
              ) : null}
            </p>
          )}
        </div>

        <ChevronRight
          className={cn(
            "mt-1 h-4 w-4 flex-none text-muted-foreground/60 transition-transform duration-200",
            isExpanded && "rotate-90",
          )}
          data-testid="tool-expand-icon"
          aria-hidden="true"
        />
      </button>

      {/* Expanded Content */}
      {isExpanded && showBody && (
        <div className="mt-2 pl-4 pr-1" data-testid="tool-content-expanded">
          <div className="rounded-xl border border-border/40 bg-muted/20 p-3">
            <div className="grid gap-3 lg:grid-cols-2">
              {hasParameters && (
                <ToolArgumentsPanel
                  args={formattedArguments}
                  label={t("tool.section.parameters")}
                  copyLabel={t("events.toolCall.copyArguments")}
                  copiedLabel={t("events.toolCall.copied")}
                />
              )}

              {(hasResult || hasError || attachmentCount > 0) && (
                <ToolResultPanel
                  toolName={toolName}
                  result={result ?? ""}
                  error={error ?? null}
                  resultTitle={t("tool.section.output")}
                  errorTitle={t("tool.section.error")}
                  copyLabel={t("events.toolCall.copyResult")}
                  copyErrorLabel={t("events.toolCall.copyError")}
                  copiedLabel={t("events.toolCall.copied")}
                  attachments={attachments}
                  metadata={(metadata as Record<string, any>) ?? null}
                />
              )}

              {hasMetadata && (
                <ToolStreamPanel
                  title={t("conversation.tool.timeline.metadata")}
                  content={formattedMetadata}
                />
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function formatParams(
  parameters?: Record<string, unknown>,
  toolName?: string,
): string | null {
  if (!parameters) return null;
  const entries = Object.entries(parameters);
  if (entries.length === 0) return null;

  if (toolName === "run_command" || toolName === 'bash') {
    return (parameters.command as string) || null;
  }

  return entries
    .slice(0, 2)
    .map(([key, value]) => `${key}: ${formatParamValue(value)}`)
    .join(", ");
}

function formatParamValue(value: unknown): string {
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
}
