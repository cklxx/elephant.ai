"use client";

import { useState, useRef, useEffect, useCallback, useMemo } from "react";
import Image from "next/image";
import { ImageUp, Send, Square, X } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { AttachmentUpload } from "@/lib/types";

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
    if (typeof prefill !== "string") return;
    const nextValue = prefill.trim();
    if (!nextValue) return;

    setTask(prefill);

    const focusField = () => {
      if (!textareaRef.current) return;
      textareaRef.current.focus();
      const length = prefill.length;
      textareaRef.current.setSelectionRange(length, length);
    };

    if (
      typeof window !== "undefined" &&
      typeof window.requestAnimationFrame === "function"
    ) {
      window.requestAnimationFrame(focusField);
    } else {
      setTimeout(focusField, 0);
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

      const images: File[] = [];
      for (let i = 0; i < items.length; i += 1) {
        const item = items[i];
        if (item.kind === "file") {
          const file = item.getAsFile();
          if (file && file.type.startsWith("image/")) {
            images.push(file);
          }
        }
      }

      if (!images.length) {
        return;
      }

      event.preventDefault();
      const text = event.clipboardData?.getData("text");
      if (text) {
        insertContentAtCursor(text);
      }
      await processFiles(images);
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

  const openFilePicker = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

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

  const attachLabel = translateWithFallback(
    "task.input.attachImage",
    undefined,
    "Attach file",
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

  return (
    <form
      onSubmit={handleSubmit}
      className="w-full rounded-3xl bg-white/5 p-4 shadow-none backdrop-blur-xl"
      data-testid="task-input-form"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:gap-3">
        <div className="relative flex-1 sm:items-center">
          <textarea
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
            className="min-h-[2.75rem] max-h-32 w-full resize-none overflow-y-auto rounded-2xl border border-white/40 bg-white/60 px-3.5 pr-12 py-2.5 text-[13px] text-foreground placeholder:text-foreground/60 shadow-none backdrop-blur focus:border-white focus:outline-none focus:ring-2 focus:ring-white/40 disabled:cursor-not-allowed disabled:opacity-60"
            style={{ fieldSizing: "content" } as any}
          />
          <button
            type="button"
            onClick={openFilePicker}
            disabled={isInputDisabled}
            className={cn(
              "absolute right-2 top-1/2 inline-flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-full bg-white/20 text-foreground/70 backdrop-blur transition hover:bg-white/30 disabled:cursor-not-allowed disabled:opacity-50",
            )}
            title={attachLabel}
            aria-label={attachLabel}
            data-testid="task-attach-image"
          >
            <ImageUp className="h-4 w-4" />
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept={acceptedFileTypes}
            multiple
            className="hidden"
            onChange={handleFileInputChange}
          />
        </div>

        {showStopButton ? (
          <button
            type="button"
            onClick={onStop}
            disabled={stopButtonDisabled}
            className={cn(
              "console-primary-action h-[2.75rem]",
              "bg-destructive/85 text-destructive-foreground hover:bg-destructive/90",
              "disabled:bg-destructive/70 disabled:text-destructive-foreground",
            )}
            title={t("task.stop.title")}
            data-testid="task-stop"
          >
            {stopPending ? (
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 animate-pulse rounded-full bg-white/80" />
                {t("task.stop.pending")}
              </span>
            ) : (
              <span className="flex items-center gap-1.5">
                <Square className="h-3.5 w-3.5" />
                {t("task.stop.label")}
              </span>
            )}
          </button>
        ) : (
          <button
            type="submit"
            disabled={isInputDisabled || !task.trim()}
            className="console-primary-action h-[2.75rem]"
            title={
              loading
                ? t("task.submit.title.running")
                : t("task.submit.title.default")
            }
            data-testid="task-submit"
          >
            {loading ? (
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 animate-pulse rounded-full bg-white/80" />
                {t("task.submit.running")}
              </span>
            ) : (
              <span className="flex items-center gap-1.5">
                <Send className="h-3.5 w-3.5" />
                {t("task.submit.label")}
              </span>
            )}
          </button>
        )}
      </div>

      {attachments.length > 0 && (
        <div
          className="mt-3 flex flex-col gap-3"
          data-testid="task-attachments"
        >
          {attachments.map((attachment) => (
            <div
              key={attachment.id}
              className="flex flex-col overflow-hidden rounded-2xl bg-white/10 shadow-none backdrop-blur sm:flex-row"
            >
              <div className="relative flex h-36 w-full items-center justify-center bg-white/10 sm:h-auto sm:w-32">
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
                  <span className="px-2 text-center text-[11px] font-semibold uppercase tracking-wide text-foreground/60">
                    {attachment.format
                      ? attachment.format.slice(0, 6).toUpperCase()
                      : noPreviewLabel}
                  </span>
                )}
                <button
                  type="button"
                  onClick={() => handleRemoveAttachment(attachment.id)}
                  className="absolute right-2 top-2 inline-flex h-6 w-6 items-center justify-center rounded-full bg-black/40 text-white transition hover:bg-black/70"
                  aria-label={getRemoveLabel(attachment.name)}
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
              <div className="flex flex-1 flex-col gap-1 px-3 py-3 text-[11px] text-foreground/80">
                <div className="text-sm font-semibold text-foreground">
                  {attachment.name}
                </div>
                <div>{attachment.mediaType}</div>
                <div className="text-foreground/60">{formatFileSize(attachment.size)}</div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {(["attachment", "artifact"] as AttachmentKind[]).map((kind) => {
                    const isActive = attachment.kind === kind;
                    return (
                      <button
                        key={kind}
                        type="button"
                        onClick={() => handleAttachmentKindChange(attachment.id, kind)}
                        className={cn(
                          "rounded-full px-3 py-1 text-[10px] font-semibold uppercase tracking-wide text-foreground transition backdrop-blur",
                          isActive
                            ? "bg-foreground text-primary-foreground shadow-none"
                            : "bg-white/15 text-foreground hover:bg-white/25",
                        )}
                      >
                        {attachmentKindLabels[kind]}
                      </button>
                    );
                  })}
                </div>
                {attachment.kind === "artifact" && (
                  <p className="mt-1 text-[10px] text-foreground/70">{artifactHint}</p>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="mt-2 flex justify-end text-[10px] font-medium uppercase tracking-[0.35em] text-foreground/50">
        {t("console.input.hotkeyHint")}
      </div>
    </form>
  );
}
