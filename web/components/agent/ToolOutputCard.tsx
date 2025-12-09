"use client";

import { useMemo, useState } from "react";
import Image from "next/image";
import { Badge, type BadgeProps } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatDuration, cn, humanizeToolName } from "@/lib/utils";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";
import { ChevronRight, Loader2, Check, X } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { AttachmentPayload } from "@/lib/types";
import { parseContentSegments, buildAttachmentUri } from "@/lib/attachments";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "./ArtifactPreviewCard";
import { startCase } from "lodash";

const SEEDREAM_TOOL_ALIASES = new Set([
  'video_generate',
  'text_to_image',
  'image_to_image',
  'vision_analyze',
]);

function isSeedreamTool(toolName: string) {
  const normalized = toolName.toLowerCase().trim();
  return normalized.includes('seedream') || SEEDREAM_TOOL_ALIASES.has(normalized);
}

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

  const language = useMemo(
    () => detectLanguage(toolName, parameters, result),
    [toolName, parameters, result],
  );

  const previewText = useMemo(() => {
    if (error) return error;
    if (result) {
      const trimmed = result.trim();
      return trimmed.length > 100 ? trimmed.slice(0, 100) + '...' : trimmed;
    }
    // Fallback to params
    return formatParams(parameters, toolName) || "";
  }, [error, result, parameters, toolName]);

  const showBody = hasResult || hasParameters || hasError || (metadata?.todos);

  return (
    <div
      className="group mb-2 transition-all"
      data-testid={`tool-output-card-${normalizedToolName.replace(/\s+/g, '-')}`}
    >
      {/* Manus Style Gray Pill Header */}
      <div
        role="button"
        onClick={() => setIsExpanded(!isExpanded)}
        data-testid="tool-output-header"
        className={cn(
          "flex items-center gap-3 px-3 py-2 cursor-pointer select-none rounded-[10px] text-sm",
          "bg-secondary/40 hover:bg-secondary/60 transition-colors border border-transparent",
          resolvedStatus === 'running' && "bg-blue-50/50 border-blue-100/50 text-blue-900",
          resolvedStatus === 'failed' && "bg-red-50/50 border-red-100/50 text-red-900"
        )}
      >
        <div
          className={cn(
            "flex items-center justify-center transition-all",
            resolvedStatus === 'running' ? "text-blue-600" :
              resolvedStatus === 'failed' ? "text-red-600" :
                "text-muted-foreground/70"
          )}
          data-testid={`tool-status-${resolvedStatus}`}
        >
          {resolvedStatus === 'running' ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> :
            resolvedStatus === 'failed' ? <X className="w-3.5 h-3.5" /> :
              <Check className="w-3.5 h-3.5" />}
        </div>

        <div className="flex-1 min-w-0 flex items-center gap-2 overflow-hidden">
          <span className="font-medium opacity-90 truncate" data-testid="tool-name">{displayToolName}</span>
        </div>

        <div className="flex items-center gap-2 text-xs text-muted-foreground/50 opacity-0 group-hover:opacity-100 transition-opacity">
          {duration && <span data-testid="tool-duration">{formatDuration(duration)}</span>}
          <ChevronRight
            className={cn("w-3.5 h-3.5 transition-transform duration-200", isExpanded && "rotate-90")}
            data-testid="tool-expand-icon"
          />
        </div>
      </div>

      {/* Expanded Content */}
      {isExpanded && showBody && (
        <div className="mt-2 pl-4 pr-1" data-testid="tool-content-expanded">
          <div className="text-sm rounded-lg overflow-hidden border border-border/40 bg-muted/10">
            <div className="p-3">
              {hasError && (
                <div className="text-xs text-destructive bg-destructive/10 p-2 rounded mb-2 font-mono whitespace-pre-wrap">
                  {error}
                </div>
              )}
              {renderToolResult(toolName, result, parameters, metadata, language, t, attachments)}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ... Keep helper functions mostly as is, just ensure they return clean JSX

function detectLanguage(
  toolName: string,
  parameters?: Record<string, unknown>,
  result?: string,
): string {
  if (toolName === "bash" || toolName === "shell" || toolName === "terminal" || toolName === "run_command") {
    return "bash";
  }
  if (toolName === "code_execute" || toolName === "python_execute") {
    return "python";
  }
  return "text";
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

function renderToolResult(
  toolName: string,
  result: string | undefined,
  parameters: Record<string, unknown> | undefined,
  metadata: Record<string, any> | undefined,
  language: string,
  t: any,
  attachments?: Record<string, AttachmentPayload>,
): React.ReactNode {
  // If we have attachments (images etc), render them first
  if (attachments && Object.keys(attachments).length > 0) {
    return (
      <div className="space-y-2">
        {Object.values(attachments).map((att, i) => (
          <ArtifactPreviewCard key={i} attachment={att} />
        ))}
        {result && <pre className="text-xs font-mono whitespace-pre-wrap mt-2">{result}</pre>}
      </div>
    );
  }

  if (toolName === 'code_execute' || toolName === 'run_command' || toolName === 'bash') {
    const code = parameters?.code || parameters?.command;
    return (
      <div className="space-y-2 text-xs">
        {!!code && (
          <div className="bg-muted/30 p-2 rounded">
            <div className="opacity-50 mb-1">Input:</div>
            <code className="font-mono whitespace-pre-wrap text-foreground/80">
              {String(code)}
            </code>
          </div>
        )}
        {result && (
          <div className="bg-muted/30 p-2 rounded">
            <div className="opacity-50 mb-1">Output:</div>
            <code className="font-mono whitespace-pre-wrap text-foreground/80">
              {result}
            </code>
          </div>
        )}
      </div>
    );
  }

  if (!result) return <span className="text-muted-foreground italic">Completed</span>;

  return (
    <pre className="text-xs font-mono whitespace-pre-wrap text-foreground/80 overflow-x-auto">
      {result}
    </pre>
  );
}

// Helper to guess language from path
function detectLanguageFromPath(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase();
  if (ext === 'js' || ext === 'jsx') return 'javascript';
  if (ext === 'ts' || ext === 'tsx') return 'typescript';
  if (ext === 'py') return 'python';
  return 'text';
}

function getFirstSeedreamDescriptor(candidates: (string | undefined)[]) {
  return candidates.find(c => c && c.trim().length > 0) || "Image Generation";
}
