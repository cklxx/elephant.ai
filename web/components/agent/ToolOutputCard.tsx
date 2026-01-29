"use client";

import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { formatDuration, cn, getToolIcon } from "@/lib/utils";
import { ChevronRight, Loader2 } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { AttachmentPayload } from "@/lib/types";
import { isDebugModeEnabled } from "@/lib/debugMode";
import { userFacingToolTitle, userFacingToolSummary } from "@/lib/toolPresentation";
import { useElapsedDurationMs } from "@/hooks/useElapsedDurationMs";
import { sanitizeToolMetadataForUI } from "@/lib/toolSanitize";
import {
  ToolArgumentsPanel,
  ToolResultPanel,
  ToolStreamPanel,
} from "./tooling/ToolPanels";

interface ToolOutputCardProps {
  toolName: string;
  parameters?: Record<string, unknown>;
  result?: unknown;
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
  const sanitizedMetadata = useMemo(
    () => sanitizeToolMetadataForUI(toolName, metadata ?? null) ?? null,
    [toolName, metadata],
  );
  const resultText = normalizeToolResult(result);
  const hasResult = resultText.trim().length > 0;
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
    return userFacingToolTitle({
      toolName,
      arguments: (parameters as Record<string, any>) ?? null,
      metadata: (metadata as Record<string, any>) ?? null,
      attachments: attachments ?? null,
    });
  }, [attachments, metadata, parameters, toolName]);

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

  const debugMode = isDebugModeEnabled();

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

  const attachmentCount = useMemo(
    () => (attachments ? Object.keys(attachments).length : 0),
    [attachments],
  );

  const summaryText = useMemo(() => {
    const resultText = normalizeToolResult(result);
    return userFacingToolSummary({
      toolName,
      result: resultText,
      error: error ?? null,
      metadata: sanitizedMetadata ?? null,
      attachments: attachments ?? null,
    });
  }, [toolName, result, error, sanitizedMetadata, attachments]);

  const hasMetadata =
    Boolean(sanitizedMetadata) &&
    typeof sanitizedMetadata === "object" &&
    Object.keys(sanitizedMetadata ?? {}).length > 0;
  const showBody =
    hasResult ||
    hasParameters ||
    hasError ||
    hasMetadata ||
    attachmentCount > 0;

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
      return JSON.stringify(sanitizedMetadata, null, 2);
    } catch {
      return String(sanitizedMetadata);
    }
  }, [hasMetadata, sanitizedMetadata]);

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

  return (
    <div
      className="group"
      data-testid={`tool-output-card-${normalizedToolName.replace(/\s+/g, "-")}`}
    >
      <button
        type="button"
        onClick={() => setIsExpanded((prev) => !prev)}
        aria-expanded={isExpanded}
        data-testid="tool-output-header"
        title={toggleLabel}
        className={cn(
          "grid grid-cols-[16px,auto,1fr,auto] items-center gap-x-2 px-3 py-1.5 text-left w-full",
          "text-[13px] leading-snug",
          "cursor-pointer select-none rounded-md border border-border/60",
          "bg-muted/50 transition-colors hover:bg-muted/70",
          resolvedStatus === "running" &&
            "bg-blue-50/50 border-blue-100/50 text-blue-900 dark:bg-blue-900/20 dark:text-blue-100 dark:border-blue-800/30",
          resolvedStatus === "failed" &&
            "bg-red-50/50 border-red-100/50 text-red-900 dark:bg-red-900/20 dark:text-red-100 dark:border-red-800/30",
        )}
      >
        <div
          className={cn(
            "flex h-4 w-4 items-center justify-center",
            resolvedStatus === "running" && "text-blue-600 dark:text-blue-400",
            resolvedStatus === "failed" && "text-red-600 dark:text-red-400",
            resolvedStatus === "completed" && "text-muted-foreground/70",
          )}
        >
          {resolvedStatus === "running" ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <span className="text-[10px] leading-none" aria-hidden="true">
              {toolIcon}
            </span>
          )}
        </div>

        <ChevronRight
          className={cn(
            "h-3.5 w-3.5 text-muted-foreground/50 transition-transform duration-200",
            isExpanded && "rotate-90",
          )}
        />

        <div className="min-w-0 overflow-hidden">
          <span
            className={cn(
              "block truncate text-[13px] font-semibold tracking-tight",
              resolvedStatus === "completed" && "text-muted-foreground/80",
            )}
            data-testid="tool-name"
          >
            {displayToolName}
          </span>
          {summaryText && !isExpanded ? (
            <span className="block truncate text-[12px] text-muted-foreground/60">
              {summaryText}
            </span>
          ) : null}
          {debugMode && callId && (
            <p className="text-[10px] tabular-nums text-muted-foreground/60">
              {t("events.toolCall.id")}: {callId}
            </p>
          )}
        </div>

        <div className="flex items-center gap-2 text-[11px] tabular-nums text-muted-foreground/60">
          {displayDurationMs != null ? (
            <span>{formatDuration(displayDurationMs)}</span>
          ) : null}
        </div>
      </button>

      {/* Expanded Content — CSS grid-row animation */}
      <div
        className={cn(
          "grid transition-[grid-template-rows] duration-200 ease-out",
          isExpanded && showBody ? "grid-rows-[1fr]" : "grid-rows-[0fr]",
        )}
      >
        <div
          className={cn(
            "overflow-hidden transition-opacity duration-150",
            isExpanded && showBody ? "opacity-100" : "opacity-0",
          )}
        >
          <div className="mt-2 pl-4 pr-1 border-l-2 border-border/40 ml-2" data-testid="tool-content-expanded">
            <div className="grid gap-3 grid-cols-1 lg:grid-cols-[repeat(auto-fit,minmax(320px,1fr))]">
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
                  metadata={(sanitizedMetadata as Record<string, any>) ?? null}
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
      </div>
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

  if (toolName === "run_command" || toolName === "bash") {
    return (parameters.command as string) || null;
  }

  return entries
    .slice(0, 2)
    .map(([key, value]) => `${key}: ${formatParamValue(value)}`)
    .join(", ");
}

function formatParamValue(value: unknown): string {
  if (typeof value === "string") return value;
  return JSON.stringify(value);
}

function normalizeToolResult(result: unknown): string {
  if (typeof result === "string") {
    return result;
  }
  if (result == null) {
    return "";
  }
  try {
    return JSON.stringify(result, null, 2);
  } catch {
    return String(result);
  }
}
