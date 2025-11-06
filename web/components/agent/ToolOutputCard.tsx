"use client";

import { useMemo, useState } from "react";
import Image from "next/image";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatDuration, cn } from "@/lib/utils";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";
import { ChevronDown, ChevronUp } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { formatTimestamp } from "./EventLine/formatters";
import { AttachmentPayload } from "@/lib/types";
import { parseContentSegments, buildAttachmentUri } from "@/lib/attachments";

interface ToolOutputCardProps {
  toolName: string;
  parameters?: Record<string, unknown>;
  result?: string;
  error?: string;
  duration?: number;
  timestamp: string;
  callId?: string;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload>;
}

export function ToolOutputCard({
  toolName,
  parameters,
  result,
  error,
  duration,
  metadata,
  attachments,
}: ToolOutputCardProps) {
  const hasResult = Boolean(result && result.trim().length > 0);
  const hasParameters = Boolean(
    parameters && Object.keys(parameters).length > 0,
  );
  const hasError = Boolean(error && error.trim().length > 0);
  const [isExpanded, setIsExpanded] = useState(
    () => Boolean(error) || !hasResult,
  );
  const t = useTranslation();

  const language = useMemo(
    () => detectLanguage(toolName, parameters, result),
    [toolName, parameters, result],
  );

  const resultLength = result?.length ?? 0;
  const errorLength = error?.length ?? 0;
  const shouldShowToggle =
    (hasResult && resultLength > 0) ||
    (hasError && (errorLength > 160 || hasResult));

  const previewText = useMemo(() => {
    if (error) {
      return error;
    }
    if (!result) {
      return "";
    }
    const trimmed = result.trim();
    if (trimmed.length <= 160) {
      return trimmed;
    }
    return `${trimmed.slice(0, 160)}…`;
  }, [error, result]);

  return (
    <Card className="console-card border-primary/50 animate-fadeIn overflow-hidden">
      <CardHeader className="px-5 py-4 space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-2">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 font-mono text-sm">
              <span
                className={
                  error
                    ? "text-destructive font-semibold"
                    : "text-primary font-semibold"
                }
              >
                {error ? "✗" : "▸"} {toolName}
              </span>
              {typeof duration === "number" && duration >= 0 && (
                <Badge variant="info" className="font-mono text-[11px]">
                  {formatDuration(duration)}
                </Badge>
              )}
            </div>
          </div>
          <Badge variant={error ? "error" : "success"} className="shrink-0">
            {error ? t("tool.status.failed") : t("tool.status.completed")}
          </Badge>
        </div>
      </CardHeader>

      {(hasResult ||
        hasParameters ||
        hasError ||
        ((toolName === "todo_read" || toolName === "todo_update") &&
          metadata?.todos)) && (
        <div className="border-t border-border/60 bg-muted/40">
          {shouldShowToggle && (
            <button
              type="button"
              aria-expanded={isExpanded}
              onClick={() => setIsExpanded((prev) => !prev)}
              className="flex w-full items-center justify-between gap-2 px-5 py-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted/60"
            >
              <span className="flex flex-wrap items-center gap-1">
                {isExpanded
                  ? t("tool.toggle.collapse")
                  : t("tool.toggle.expand")}
                {hasResult &&
                  t("tool.toggle.length", {
                    count: resultLength.toLocaleString(),
                  })}
              </span>
              {isExpanded ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <ChevronDown className="h-4 w-4" />
              )}
            </button>
          )}

          {(isExpanded || !shouldShowToggle) && (
            <CardContent className="space-y-4 px-5 pb-5 pt-4">
              {hasError && (
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-destructive">
                    {t("tool.section.error")}
                  </p>
                  <pre className="console-card bg-destructive/10 border-destructive/30 p-3 text-xs font-mono text-destructive overflow-x-auto">
                    {error}
                  </pre>
                </div>
              )}

              {/* Todo tools - render from metadata */}
              {(toolName === "todo_read" || toolName === "todo_update") &&
                metadata?.todos && (
                  <div className="space-y-4">
                    {/* Todo List Header with Summary */}
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-semibold text-slate-700">
                        📋 Todos
                      </h3>
                      {metadata.total_count > 0 && (
                        <div className="flex gap-3 text-xs">
                          {metadata.in_progress_count > 0 && (
                            <span className="flex items-center gap-1 rounded-full bg-blue-50 px-2 py-1 font-medium text-blue-600">
                              <span className="text-blue-500">→</span>
                              {metadata.in_progress_count}
                            </span>
                          )}
                          {metadata.pending_count > 0 && (
                            <span className="flex items-center gap-1 rounded-full bg-yellow-50 px-2 py-1 font-medium text-yellow-700">
                              <span className="text-yellow-600">☐</span>
                              {metadata.pending_count}
                            </span>
                          )}
                          {metadata.completed_count > 0 && (
                            <span className="flex items-center gap-1 rounded-full bg-green-50 px-2 py-1 font-medium text-green-600">
                              <span className="text-green-600">✓</span>
                              {metadata.completed_count}
                            </span>
                          )}
                        </div>
                      )}
                    </div>

                    {/* Todo List Items */}
                    {metadata.todos.length > 0 ? (
                      <div className="space-y-2">
                        {metadata.todos.map(
                          (
                            todo: { content: string; status: string },
                            index: number,
                          ) => (
                            <div
                              key={index}
                              className={cn(
                                "group flex items-start gap-3 rounded-lg border p-3 transition-all",
                                todo.status === "in_progress" &&
                                  "border-blue-200 bg-blue-50/50",
                                todo.status === "pending" &&
                                  "border-slate-200 bg-white hover:border-slate-300",
                                todo.status === "completed" &&
                                  "border-green-200 bg-green-50/30",
                              )}
                            >
                              {/* Status Icon */}
                              <div className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center">
                                {todo.status === "in_progress" && (
                                  <div className="flex h-5 w-5 items-center justify-center rounded-full bg-blue-500 text-white">
                                    <span className="text-sm"></span>
                                  </div>
                                )}
                                {todo.status === "pending" && (
                                  <div className="flex h-5 w-5 items-center justify-center rounded border-2 border-slate-300 bg-white">
                                    <span className="text-xs text-slate-400"></span>
                                  </div>
                                )}
                                {todo.status === "completed" && (
                                  <div className="flex h-5 w-5 items-center justify-center rounded-full bg-green-500 text-white">
                                    <span className="text-sm">✓</span>
                                  </div>
                                )}
                              </div>

                              {/* Task Content */}
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2 ">
                                  <p
                                    className={cn(
                                      "text-sm leading-relaxed",
                                      todo.status === "completed"
                                        ? "text-slate-400 line-through"
                                        : "text-slate-700",
                                    )}
                                  >
                                    {todo.content}
                                  </p>
                                </div>
                              </div>
                            </div>
                          ),
                        )}
                      </div>
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-200 bg-slate-50 p-6 text-center">
                        <p className="text-sm text-slate-400">No tasks yet</p>
                      </div>
                    )}
                  </div>
                )}

              {hasResult &&
                !(toolName === "todo_read" || toolName === "todo_update") &&
                renderToolResult(
                  toolName,
                  result,
                  parameters,
                  metadata,
                  language,
                  t,
                  attachments,
                )}

              {metadata?.screenshot && (
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    Screenshot
                  </p>
                  <div className="rounded-lg border border-border/60 overflow-hidden bg-white">
                    <Image
                      src={metadata.screenshot}
                      alt="Browser screenshot"
                      width={1280}
                      height={720}
                      className="w-full h-auto max-h-96 object-contain"
                      unoptimized
                    />
                  </div>
                </div>
              )}
            </CardContent>
          )}
        </div>
      )}
    </Card>
  );
}

function detectLanguage(
  toolName: string,
  parameters?: Record<string, unknown>,
  result?: string,
): string {
  if (toolName === "bash" || toolName === "shell" || toolName === "terminal") {
    return "bash";
  }
  if (toolName === "code_execute" || toolName === "python_execute") {
    return "python";
  }
  if (toolName === "web_fetch" || toolName === "web_search") {
    return "html";
  }
  if (toolName === "file_read") {
    const path = typeof parameters?.path === "string" ? parameters.path : "";
    const ext = path.split(".").pop();
    if (ext) {
      const normalized = ext.toLowerCase();
      if (normalized === "ts" || normalized === "tsx") return "typescript";
      if (normalized === "js" || normalized === "jsx") return "javascript";
      if (normalized === "json") return "json";
      if (normalized === "py") return "python";
      if (normalized === "go") return "go";
      if (normalized === "rs") return "rust";
      if (normalized === "sh" || normalized === "bash") return "bash";
      if (normalized === "md") return "markdown";
      if (normalized === "html" || normalized === "htm") return "html";
      if (normalized === "css") return "css";
      if (normalized === "sql") return "sql";
      if (normalized === "yml" || normalized === "yaml") return "yaml";
    }
  }

  if (result) {
    const trimmed = result.trim();
    if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
      return "json";
    }
    if (/^<([a-z-]+)(\s|>)/i.test(trimmed)) {
      return "html";
    }
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

  // Custom formatting for specific tools
  if (toolName === "file_write" || toolName === "file_edit") {
    const path = parameters.path || parameters.file_path;
    if (path && typeof path === "string") {
      const lines = parameters.lines || parameters.line_count;
      if (lines) {
        return `${path} (${lines} lines)`;
      }
      return path;
    }
  }

  if (toolName === "file_read") {
    const path = parameters.path || parameters.file_path;
    if (path && typeof path === "string") {
      return path;
    }
  }

  if (toolName === "bash" || toolName === "shell") {
    const command = parameters.command;
    if (command && typeof command === "string") {
      return command.length > 60 ? `${command.slice(0, 60)}…` : command;
    }
  }

  // Default formatting
  return entries
    .slice(0, 2)
    .map(([key, value]) => `${key}: ${formatParamValue(value)}`)
    .join(", ");
}

function formatParamValue(value: unknown): string {
  if (value == null) return "null";
  if (typeof value === "string") {
    return value.length > 24 ? `${value.slice(0, 24)}…` : value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value).slice(0, 24);
}

// Parse bash tool result
interface BashResult {
  command?: string;
  exit_code?: number;
  stdout?: string;
  stderr?: string;
  text?: string;
}

function parseBashResult(
  result: string | undefined,
  metadata?: Record<string, any>,
): BashResult | null {
  if (typeof result === "string" && result.trim().length > 0) {
    try {
      const parsed = JSON.parse(result);
      return parsed as BashResult;
    } catch {
      // fall through to metadata fallback
    }
  }

  if (!metadata) {
    return null;
  }

  const command =
    typeof metadata.command === "string" ? metadata.command : undefined;
  const stdout =
    typeof metadata.stdout === "string" ? metadata.stdout : undefined;
  const stderr =
    typeof metadata.stderr === "string" ? metadata.stderr : undefined;
  const text = typeof metadata.text === "string" ? metadata.text : undefined;

  let exitCode: number | undefined;
  const rawExit = metadata.exit_code;
  if (typeof rawExit === "number") {
    exitCode = rawExit;
  } else if (typeof rawExit === "string") {
    const parsedExit = Number(rawExit);
    if (!Number.isNaN(parsedExit)) {
      exitCode = parsedExit;
    }
  }

  if (
    command === undefined &&
    stdout === undefined &&
    stderr === undefined &&
    text === undefined &&
    exitCode === undefined
  ) {
    return null;
  }

  return {
    command,
    stdout,
    stderr,
    text,
    exit_code: exitCode,
  };
}

// Parse file_write tool result
interface FileWriteResult {
  path?: string;
  chars?: number;
  lines?: number;
}

function parseFileWriteResult(result: string): FileWriteResult | null {
  try {
    const parsed = JSON.parse(result);
    return parsed as FileWriteResult;
  } catch {
    return null;
  }
}

// Render tool-specific result
function renderToolResult(
  toolName: string,
  result: string | undefined,
  parameters: Record<string, unknown> | undefined,
  metadata: Record<string, any> | undefined,
  language: string,
  t: any,
  attachments?: Record<string, AttachmentPayload>,
): React.ReactNode {
  if (!result) return null;

  // Bash tool - show command and output separately
  if (toolName === "bash" || toolName === "shell") {
    const bashResult = parseBashResult(result, metadata);
    if (bashResult) {
      return (
        <div className="space-y-3">
          {bashResult.command && (
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                Command
              </p>
              <div className="console-card bg-gray-900 text-emerald-400 p-3 overflow-x-auto">
                <pre className="text-xs font-mono whitespace-pre-wrap">
                  <span className="text-gray-500">$</span> {bashResult.command}
                </pre>
              </div>
            </div>
          )}
          {(bashResult.stdout || bashResult.text) && (
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                Output
              </p>
              <div className="console-card bg-gray-900 text-gray-100 p-3 overflow-x-auto">
                <pre className="text-xs font-mono whitespace-pre-wrap">
                  {bashResult.text || bashResult.stdout}
                </pre>
              </div>
            </div>
          )}
          {bashResult.stderr && (
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-destructive">
                Error Output
              </p>
              <div className="console-card bg-destructive/10 border-destructive/30 p-3 overflow-x-auto">
                <pre className="text-xs font-mono text-destructive whitespace-pre-wrap">
                  {bashResult.stderr}
                </pre>
              </div>
            </div>
          )}
          {bashResult.exit_code !== undefined && (
            <div className="flex items-center gap-2">
              <span className="text-xs font-semibold text-muted-foreground">
                Exit Code:
              </span>
              <Badge variant={bashResult.exit_code === 0 ? "success" : "error"}>
                {bashResult.exit_code}
              </Badge>
            </div>
          )}
        </div>
      );
    }
  }

  // File write tool - show file path and write summary
  if (toolName === "file_write") {
    // Get file path from metadata or parameters
    const filePath =
      (metadata?.path as string) ||
      (parameters?.path as string) ||
      (parameters?.file_path as string);
    // Get content from metadata (preferred) or parameters (fallback)
    const content =
      (metadata?.content as string) ||
      (parameters?.content as string) ||
      undefined;

    return (
      <div className="space-y-3">
        {/* Output message */}
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Output
          </p>
          <div className="console-card bg-background p-3">
            <p className="text-xs font-mono text-foreground/90">{result}</p>
          </div>
        </div>

        {/* File path */}
        {filePath && (
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              File Path
            </p>
            <div className="console-card bg-background p-3">
              <p className="text-xs font-mono text-foreground/90">{filePath}</p>
            </div>
          </div>
        )}

        {/* Write summary from metadata */}
        {metadata &&
          (metadata.lines !== undefined || metadata.chars !== undefined) && (
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                Write Summary
              </p>
              <div className="flex gap-4 text-xs">
                {metadata.lines !== undefined && (
                  <span className="text-muted-foreground">
                    <strong>{metadata.lines}</strong> lines
                  </span>
                )}
                {metadata.chars !== undefined && (
                  <span className="text-muted-foreground">
                    <strong>{metadata.chars}</strong> characters
                  </span>
                )}
              </div>
            </div>
          )}

        {/* File content from parameters */}
        {content && (
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              Content Written
            </p>
            <div className="rounded-lg border border-border/60 overflow-auto">
              <SyntaxHighlighter
                language={detectLanguageFromPath(filePath || "")}
                style={vscDarkPlus}
                customStyle={{
                  margin: 0,
                  borderRadius: "0.5rem",
                  fontSize: "0.75rem",
                  maxHeight: 400,
                }}
                showLineNumbers={true}
              >
                {content}
              </SyntaxHighlighter>
            </div>
          </div>
        )}
      </div>
    );
  }

  // Default rendering for other tools
  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {t("tool.section.output")}
      </p>
      {attachments && Object.keys(attachments).length > 0 ? (
        <div className="rounded-lg border border-border/60 overflow-auto p-3 bg-background">
          <div className="whitespace-pre-wrap font-mono text-xs space-y-3">
            {parseContentSegments(result ?? "", attachments).map((segment, index) => {
              if (segment.type === "image" && segment.attachment) {
                const uri = buildAttachmentUri(segment.attachment);
                if (!uri) {
                  return (
                    <span key={`segment-${index}`}>{segment.placeholder ?? ""}</span>
                  );
                }
                return (
                  <figure
                    key={`segment-${index}`}
                    className="flex flex-col items-start gap-1"
                  >
                    <div
                      className="relative w-full overflow-hidden rounded border border-border/60 bg-white"
                      style={{ minHeight: "10rem", maxHeight: "16rem" }}
                    >
                      <Image
                        src={uri}
                        alt={segment.attachment.description || segment.attachment.name}
                        fill
                        className="object-contain"
                        sizes="(min-width: 1280px) 25vw, (min-width: 768px) 40vw, 100vw"
                        unoptimized
                      />
                    </div>
                    <figcaption className="text-[10px] uppercase tracking-wide text-muted-foreground">
                      {segment.attachment.description || segment.attachment.name}
                    </figcaption>
                  </figure>
                );
              }

              return (
                <span key={`segment-${index}`}>{segment.text}</span>
              );
            })}
          </div>
        </div>
      ) : (
        <div className="rounded-lg border border-border/60 overflow-auto">
          <SyntaxHighlighter
            language={language}
            style={vscDarkPlus}
            customStyle={{
              margin: 0,
              borderRadius: "0.5rem",
              fontSize: "0.75rem",
              maxHeight: 400,
            }}
            showLineNumbers={language !== "text"}
          >
            {result}
          </SyntaxHighlighter>
        </div>
      )}
    </div>
  );
}

// Helper to detect language from file path
function detectLanguageFromPath(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase();
  if (!ext) return "text";

  const langMap: Record<string, string> = {
    ts: "typescript",
    tsx: "typescript",
    js: "javascript",
    jsx: "javascript",
    py: "python",
    go: "go",
    rs: "rust",
    sh: "bash",
    bash: "bash",
    md: "markdown",
    json: "json",
    html: "html",
    htm: "html",
    css: "css",
    scss: "scss",
    yaml: "yaml",
    yml: "yaml",
    sql: "sql",
    xml: "xml",
  };

  return langMap[ext] || "text";
}
