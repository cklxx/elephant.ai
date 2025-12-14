"use client";

import { useState, useRef, useEffect, useCallback, useMemo } from "react";
import Image from "next/image";
import { ArrowUp, Paperclip, Square, X, Loader2 } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { AttachmentUpload } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";

interface TaskInputProps {
  onSubmit: (task: string, attachments: AttachmentUpload[]) => void;
  disabled?: boolean;
  loading?: boolean;
  placeholder?: string;
  prefill?: string | null;
  onPrefillApplied?: () => void;
  onStop?: () => void;
  isRunning?: boolean;
  stopPending?: boolean;
  stopDisabled?: boolean;
}

type AttachmentKind = "attachment" | "artifact";

type PendingAttachment = {
  id: string;
  name: string;
  mediaType: string;
  data?: string;
  uri?: string;
  previewUrl?: string;
  placeholder: string;
  description?: string;
  format?: string;
  size: number;
  kind: AttachmentKind;
  retentionSeconds: number;
  isImage: boolean;
};

const artifactRetentionDays = 90;
const artifactRetentionSeconds = artifactRetentionDays * 24 * 60 * 60;
const acceptedFileTypes = [
  "image/*",
  ".pdf",
  ".ppt",
  ".pptx",
  ".html",
  ".htm",
  ".md",
  ".markdown",
  ".txt",
].join(",");

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}

function inferExtension(file: File): string {
  const fromName = file.name?.split(".").pop();
  if (fromName && /^[a-zA-Z0-9]{1,5}$/.test(fromName)) {
    return fromName.toLowerCase();
  }
  const fromType = file.type?.split("/").pop();
  if (fromType && /^[a-zA-Z0-9]{1,5}$/.test(fromType)) {
    return fromType.toLowerCase();
  }
  return "png";
}

function isPreviewableImage(mediaType: string): boolean {
  return mediaType.startsWith("image/") && mediaType !== "image/svg+xml";
}

function formatFileSize(size: number): string {
  if (!Number.isFinite(size) || size <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB"];
  let idx = 0;
  let value = size;
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024;
    idx += 1;
  }
  return `${value.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

function resolveMediaType(file: File, ext: string): string {
  if (file.type) return file.type;
  if (!ext) return "application/octet-stream";
  switch (ext.toLowerCase()) {
    case "md":
    case "markdown":
      return "text/markdown";
    case "html":
    case "htm":
      return "text/html";
    case "ppt":
      return "application/vnd.ms-powerpoint";
    case "pptx":
      return "application/vnd.openxmlformats-officedocument.presentationml.presentation";
    case "pdf":
      return "application/pdf";
    default:
      return `application/${ext}`;
  }
}

function sanitizeBaseName(file: File): string {
  const raw = file.name?.split(".").slice(0, -1).join(".") ?? "";
  const trimmed = raw.trim();
  if (!trimmed) {
    return "image";
  }
  const normalized = trimmed
    .replace(/[^a-zA-Z0-9-_]+/g, "-")
    .replace(/-{2,}/g, "-");
  const cleaned = normalized.replace(/^-+|-+$/g, "");
  return cleaned || "image";
}

function createId(): string {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  return `att-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function collapseWhitespaceAroundPlaceholder(
  content: string,
  placeholder: string,
): string {
  if (!content.includes(placeholder)) {
    return content;
  }
  const index = content.indexOf(placeholder);
  const before = content.slice(0, index).replace(/[ \t]+$/g, " ");
  const after = content
    .slice(index + placeholder.length)
    .replace(/^[ \t]+/g, " ");
  return `${before}${after}`
    .replace(/\s{3,}/g, " ")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

export function TaskInput({
  onSubmit,
  disabled = false,
  loading = false,
  placeholder,
  prefill = null,
  onPrefillApplied,
  onStop,
  isRunning = false,
  stopPending = false,
  stopDisabled = false,
}: TaskInputProps) {
  const [task, setTask] = useState("");
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const lastAppliedPrefillRef = useRef<string | null>(null);
  const t = useTranslation();
  const resolvedPlaceholder =
    placeholder ?? t("console.input.placeholder.idle");

  const translateWithFallback = useCallback(
    (
      key: string,
      params: Record<string, unknown> | undefined,
      fallback: string,
    ): string => {
      try {
        const value = params ? t(key as any, params as any) : t(key as any);
        if (typeof value !== "string" || value === key) {
          return fallback;
        }
        return value;
      } catch (error) {
        console.warn("[TaskInput] Missing translation", { key, error });
        return fallback;
      }
    },
    [t],
  );

  // Auto-resize textarea
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = `${textareaRef.current.scrollHeight}px`;
    }
  }, [task]);

  useEffect(() => {
    if (typeof prefill !== "string") {
      lastAppliedPrefillRef.current = null;
      return;
    }

    const nextValue = prefill.trim();
    if (!nextValue) {
      lastAppliedPrefillRef.current = null;
      return;
    }

    if (prefill === lastAppliedPrefillRef.current) {
      return;
    }
    lastAppliedPrefillRef.current = prefill;

    const textarea = textareaRef.current;
    if (textarea) {
      const setter = Object.getOwnPropertyDescriptor(
        window.HTMLTextAreaElement.prototype,
        "value",
      )?.set;
      if (setter) {
        setter.call(textarea, prefill);
      } else {
        textarea.value = prefill;
      }

      textarea.dispatchEvent(new Event("input", { bubbles: true }));
      textarea.focus();
      textarea.setSelectionRange(prefill.length, prefill.length);
    }

    onPrefillApplied?.();
  }, [prefill, onPrefillApplied]);

  const insertContentAtCursor = useCallback(
    (contentToInsert: string, { surroundWithSpaces = false } = {}) => {
      const textarea = textareaRef.current;
      if (!textarea) {
        setTask((prev) => {
          if (!prev) return contentToInsert;
          const separator = surroundWithSpaces ? " " : "";
          return `${prev}${separator}${contentToInsert}`;
        });
        return;
      }

      const { selectionStart, selectionEnd, value } = textarea;
      const start = selectionStart ?? value.length;
      const end = selectionEnd ?? value.length;
      const before = value.slice(0, start);
      const after = value.slice(end);

      const needsPrefixSpace =
        surroundWithSpaces && before.length > 0 && !/\s$/.test(before);
      const needsSuffixSpace =
        surroundWithSpaces && after.length > 0 && !/^\s/.test(after);

      const prefix = needsPrefixSpace ? " " : "";
      const suffix = needsSuffixSpace ? " " : "";
      const nextValue = `${before}${prefix}${contentToInsert}${suffix}${after}`;
      const cursorPosition =
        before.length + prefix.length + contentToInsert.length;

      setTask(nextValue);
      requestAnimationFrame(() => {
        if (!textareaRef.current) return;
        textareaRef.current.selectionStart = cursorPosition;
        textareaRef.current.selectionEnd = cursorPosition;
        textareaRef.current.focus();
      });
    },
    [],
  );

  const insertPlaceholder = useCallback(
    (placeholderText: string) => {
      insertContentAtCursor(placeholderText, { surroundWithSpaces: true });
    },
    [insertContentAtCursor],
  );

  const processFiles = useCallback(
    async (files: File[]) => {
      if (!files.length) return;

      const existing = new Set(attachments.map((item) => item.name));
      for (const file of files) {
        try {
          const dataUrl = await readFileAsDataURL(file);
          const base64 = dataUrl.split(",")[1];
          if (!base64) {
            continue;
          }

          const baseName = sanitizeBaseName(file);
          const ext = inferExtension(file);
          let candidate = `${baseName}.${ext}`;
          let counter = 1;
          while (existing.has(candidate)) {
            candidate = `${baseName}-${counter}.${ext}`;
            counter += 1;
          }
          existing.add(candidate);

          const mediaType = resolveMediaType(file, ext);
          const previewable = isPreviewableImage(mediaType);
          const pending: PendingAttachment = {
            id: createId(),
            name: candidate,
            mediaType,
            data: base64,
            previewUrl: previewable ? dataUrl : undefined,
            placeholder: `[${candidate}]`,
            format: ext,
            size: file.size,
            kind: "attachment",
            retentionSeconds: 0,
            isImage: previewable,
          };

          setAttachments((prev) => [...prev, pending]);
          insertPlaceholder(pending.placeholder);
        } catch (error) {
          console.error("[TaskInput] Failed to read attachment", error);
        }
      }
    },
    [attachments, insertPlaceholder],
  );

  const handleFileInputChange = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const { files } = event.target;
      if (!files || files.length === 0) {
        return;
      }
      await processFiles(Array.from(files));
      event.target.value = "";
    },
    [processFiles],
  );

  const handlePaste = useCallback(
    async (event: React.ClipboardEvent<HTMLTextAreaElement>) => {
      const items = event.clipboardData?.items;
      if (!items) {
        return;
      }

      const files: File[] = [];
      for (let i = 0; i < items.length; i += 1) {
        const item = items[i];
        if (item.kind === "file") {
          const file = item.getAsFile();
          if (file) {
            files.push(file);
          }
        }
      }

      if (!files.length) {
        return;
      }

      event.preventDefault();
      const text = event.clipboardData?.getData("text");
      if (text) {
        insertContentAtCursor(text);
      }
      await processFiles(files);
    },
    [insertContentAtCursor, processFiles],
  );

  const handleRemoveAttachment = useCallback(
    (id: string) => {
      const target = attachments.find((item) => item.id === id);
      if (!target) {
        return;
      }
      setAttachments((prev) => prev.filter((item) => item.id !== id));
      setTask((prev) =>
        collapseWhitespaceAroundPlaceholder(prev, target.placeholder),
      );
    },
    [attachments],
  );

  const handleAttachmentKindChange = useCallback(
    (id: string, nextKind: AttachmentKind) => {
      setAttachments((prev) =>
        prev.map((attachment) => {
          if (attachment.id !== id) {
            return attachment;
          }
          if (attachment.kind === nextKind) {
            return attachment;
          }
          return {
            ...attachment,
            kind: nextKind,
            retentionSeconds:
              nextKind === "artifact" ? artifactRetentionSeconds : 0,
          };
        }),
      );
    },
    [],
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (task.trim() && !loading && !disabled && !isRunning) {
      const uploads: AttachmentUpload[] = attachments.map((attachment) => ({
        name: attachment.name,
        media_type: attachment.mediaType,
        data: attachment.data,
        uri: attachment.uri,
        source: "user_upload",
        description: attachment.description,
        kind: attachment.kind,
        format: attachment.format,
        retention_ttl_seconds:
          attachment.retentionSeconds > 0
            ? attachment.retentionSeconds
            : undefined,
      }));
      onSubmit(task.trim(), uploads);
      setTask("");
      setAttachments([]);
    }
  };

  const isInputDisabled = disabled || loading || isRunning;
  const showStopButton = (loading || isRunning) && typeof onStop === "function";
  const stopButtonDisabled = stopDisabled || stopPending;
  const stopPendingLabel = useMemo(
    () =>
      translateWithFallback(
        "task.stop.title.pending",
        undefined,
        "Stopping",
      ),
    [translateWithFallback],
  );

  const getRemoveLabel = useCallback(
    (name: string) =>
      translateWithFallback(
        "task.input.removeAttachment",
        { name },
        `Remove attachment ${name}`,
      ),
    [translateWithFallback],
  );
  const attachmentKindLabels = useMemo(
    () => ({
      attachment: translateWithFallback(
        "task.input.attachments.kind.attachment",
        undefined,
        "Attachment",
      ),
      artifact: translateWithFallback(
        "task.input.attachments.kind.artifact",
        undefined,
        "Artifact",
      ),
    }),
    [translateWithFallback],
  );
  const artifactHint = useMemo(
    () =>
      translateWithFallback(
        "task.input.attachments.artifactHint",
        { days: artifactRetentionDays },
        `Artifacts stay available for ${artifactRetentionDays} days with inline previews.`,
      ),
    [translateWithFallback],
  );
  const noPreviewLabel = useMemo(
    () =>
      translateWithFallback(
        "task.input.attachments.noPreview",
        undefined,
        "No preview available",
      ),
    [translateWithFallback],
  );

  const openFilePicker = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  return (
    <form
      onSubmit={handleSubmit}
      className="mx-auto w-full max-w-4xl space-y-4"
      data-testid="task-input-form"
    >
      <input
        ref={fileInputRef}
        type="file"
        accept={acceptedFileTypes}
        multiple
        className="hidden"
        onChange={handleFileInputChange}
        data-testid="task-attachment-input"
      />
      <div
        className="relative flex flex-col rounded-2xl border border-neutral-300 bg-white transition-all duration-200 focus-within:border-neutral-500 focus-within:shadow-[0_4px_20px_rgba(0,0,0,0.05)] dark:bg-muted/10 dark:border-border/50 dark:focus-within:border-neutral-600"
        data-testid="task-input-container"
      >
        <Textarea
          ref={textareaRef}
          value={task}
          onChange={(e) => setTask(e.target.value)}
          onPaste={handlePaste}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              handleSubmit(e);
            }
          }}
          placeholder={resolvedPlaceholder}
          disabled={isInputDisabled}
          rows={1}
          aria-label={t("task.input.ariaLabel")}
          data-testid="task-input"
          name="taskInput"
          id="taskInput"
          className="min-h-[40px] max-h-[300px] w-full resize-none border-none bg-transparent px-3 py-2 text-base leading-6 text-neutral-900 placeholder:text-neutral-400 shadow-none outline-none focus-visible:ring-0 focus-visible:ring-offset-0"
          style={{ fieldSizing: "content" } as any}
        />

        <div className="flex items-center justify-between px-2 pb-2">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={openFilePicker}
            disabled={isInputDisabled}
            className="h-8 w-8 text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-neutral-600 rounded-lg"
            aria-label={t("task.input.attachImage")}
            data-testid="task-attachment-trigger"
          >
            <Paperclip className="h-4.5 w-4.5" />
          </Button>

          <div className="flex items-center gap-2">
            {showStopButton ? (
              <Button
                type="button"
                onClick={onStop}
                disabled={stopButtonDisabled}
                variant="destructive"
                className="h-8 w-8 rounded-lg p-0 transition-opacity hover:opacity-90"
                aria-label={stopPending ? stopPendingLabel : t("task.stop.title")}
                data-testid="task-stop"
              >
                {stopPending ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span className="sr-only">{stopPendingLabel}</span>
                  </>
                ) : (
                  <Square className="h-3.5 w-3.5 fill-current" />
                )}
              </Button>
            ) : (
              <Button
                type="submit"
                disabled={isInputDisabled || !task.trim()}
                className={cn(
                  "h-8 w-8 rounded-lg p-0 transition-all",
                  task.trim() ? "bg-neutral-900 text-white hover:opacity-90" : "bg-neutral-100 text-neutral-300"
                )}
                aria-label={
                  loading
                    ? t("task.submit.title.running")
                    : t("task.submit.title.default")
                }
                data-testid="task-submit"
              >
                {loading ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <ArrowUp className="h-4 w-4" />
                )}
              </Button>
            )}
          </div>
        </div>
      </div>
      {/* Attachments remain similar but cleaner */}

      {attachments.length > 0 && (
        <div className="flex flex-col gap-3" data-testid="task-attachments">
          {attachments.map((attachment) => (
            <div
              key={attachment.id}
              className="flex flex-col overflow-hidden rounded-2xl border border-border bg-card sm:flex-row"
            >
              <div className="relative flex h-36 w-full items-center justify-center bg-neutral-50 sm:h-auto sm:w-32">
                {attachment.isImage && attachment.previewUrl ? (
                  <Image
                    src={attachment.previewUrl}
                    alt={attachment.name}
                    fill
                    className="object-cover"
                    sizes="128px"
                    unoptimized
                  />
                ) : (
                  <span className="px-2 text-center text-[11px] font-semibold text-neutral-500">
                    {attachment.format
                      ? attachment.format.slice(0, 6).toUpperCase()
                      : noPreviewLabel}
                  </span>
                )}
                <button
                  type="button"
                  onClick={() => handleRemoveAttachment(attachment.id)}
                  className="absolute right-2 top-2 inline-flex h-6 w-6 items-center justify-center rounded-full bg-neutral-900/70 text-white transition hover:bg-neutral-900"
                  aria-label={getRemoveLabel(attachment.name)}
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
              <div className="flex flex-1 flex-col gap-1 px-3 py-3 text-[11px] text-neutral-700 sm:px-4 sm:py-3.5">
                <div className="text-sm font-semibold text-neutral-900">
                  {attachment.name}
                </div>
                <div className="text-neutral-600">{attachment.mediaType}</div>
                <div className="text-neutral-500">
                  {formatFileSize(attachment.size)}
                </div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {(["attachment", "artifact"] as AttachmentKind[]).map(
                    (kind) => {
                      const isActive = attachment.kind === kind;
                      return (
                        <Button
                          key={kind}
                          type="button"
                          size="sm"
                          variant={isActive ? "default" : "outline"}
                          onClick={() =>
                            handleAttachmentKindChange(attachment.id, kind)
                          }
                          className="h-8 px-3 text-[10px] font-medium"
                        >
                          {attachmentKindLabels[kind]}
                        </Button>
                      );
                    },
                  )}
                </div>
                {attachment.kind === "artifact" && (
                  <p className="mt-1 text-[10px] text-neutral-600">
                    {artifactHint}
                  </p>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </form>
  );
}
