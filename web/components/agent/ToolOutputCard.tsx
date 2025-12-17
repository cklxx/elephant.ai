"use client";

import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { formatDuration, cn, humanizeToolName, getToolIcon } from "@/lib/utils";
import { ChevronRight, Loader2 } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { AttachmentPayload } from "@/lib/types";
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

  const previewText = useMemo(() => {
    if (error) return error;
    if (result) {
      const trimmed = result.trim();
      return trimmed.length > 100 ? trimmed.slice(0, 100) + '...' : trimmed;
    }
    // Fallback to params
    return formatParams(parameters, toolName) || "";
  }, [error, result, parameters, toolName]);

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

  const statusDotClass = useMemo(() => {
    switch (resolvedStatus) {
      case "running":
        return "bg-sky-500 animate-pulse";
      case "failed":
        return "bg-destructive";
      default:
        return "bg-emerald-500";
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
      className="group mb-2 transition-all"
      data-testid={`tool-output-card-${normalizedToolName.replace(/\s+/g, '-')}`}
    >
      <button
        type="button"
        onClick={() => setIsExpanded((prev) => !prev)}
        aria-expanded={isExpanded}
        data-testid="tool-output-header"
        title={toggleLabel}
        className={cn(
          "flex w-full items-start gap-3 px-3 py-2 text-left text-sm",
          "cursor-pointer select-none rounded-2xl border border-border/50",
          "bg-background/40 shadow-sm shadow-black/[0.02] backdrop-blur-sm",
          "transition-colors hover:bg-background/60",
          resolvedStatus === "running" && "border-sky-200/60 bg-sky-50/30",
          resolvedStatus === "failed" && "border-destructive/25 bg-destructive/5",
        )}
      >
        <div className="relative mt-0.5 flex h-9 w-9 flex-none items-center justify-center rounded-full bg-background ring-1 ring-border/60">
          <span className="text-base leading-none" aria-hidden="true">
            {toolIcon}
          </span>
          <span
            className={cn(
              "absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full ring-2 ring-background",
              statusDotClass,
            )}
            aria-hidden="true"
          />
        </div>

        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-foreground/90" data-testid="tool-name">
              {displayToolName}
            </span>
            <Badge variant={statusBadgeVariant} className="text-[10px]">
              {resolvedStatus === "running" ? (
                <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
              ) : null}
              {statusLabel}
            </Badge>
            {typeof duration === "number" && duration > 0 && (
              <Badge variant="outline" className="text-[10px] text-muted-foreground">
                {formatDuration(duration)}
              </Badge>
            )}
            {attachmentCount > 0 && (
              <Badge variant="secondary" className="text-[10px] text-muted-foreground">
                {attachmentCount} attachment{attachmentCount === 1 ? "" : "s"}
              </Badge>
            )}
          </div>

          {previewText ? (
            <p className="text-xs leading-relaxed text-muted-foreground" data-testid="tool-preview">
              {previewText}
            </p>
          ) : null}

          {(callId || timestamp) && (
            <p className="flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground/70">
              {callId ? (
                <span className="font-mono">
                  {t("events.toolCall.id")}: {callId}
                </span>
              ) : null}
              {timestampLabel ? (
                <span className="font-mono">{timestampLabel}</span>
              ) : null}
            </p>
          )}
        </div>

        <ChevronRight
          className={cn(
            "mt-2 h-4 w-4 flex-none text-muted-foreground/60 transition-transform duration-200",
            isExpanded && "rotate-90",
          )}
          data-testid="tool-expand-icon"
          aria-hidden="true"
        />
      </button>

      {/* Expanded Content */}
      {isExpanded && showBody && (
        <div className="mt-3 pl-4 pr-1" data-testid="tool-content-expanded">
          <div className="rounded-2xl border border-border/50 bg-background/60 p-3 shadow-sm shadow-black/[0.02]">
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
                  result={result ?? ""}
                  error={error ?? null}
                  resultTitle={t("tool.section.output")}
                  errorTitle={t("tool.section.error")}
                  copyLabel={t("events.toolCall.copyResult")}
                  copyErrorLabel={t("events.toolCall.copyError")}
                  copiedLabel={t("events.toolCall.copied")}
                  attachments={attachments}
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
