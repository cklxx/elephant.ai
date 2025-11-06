'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import Image from 'next/image';
import { ImageUp, Send, Square, X } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { AttachmentUpload } from '@/lib/types';

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

type PendingAttachment = {
  id: string;
  name: string;
  mediaType: string;
  data?: string;
  uri?: string;
  previewUrl: string;
  placeholder: string;
  description?: string;
};

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}

function inferExtension(file: File): string {
  const fromName = file.name?.split('.').pop();
  if (fromName && /^[a-zA-Z0-9]{1,5}$/.test(fromName)) {
    return fromName.toLowerCase();
  }
  const fromType = file.type?.split('/').pop();
  if (fromType && /^[a-zA-Z0-9]{1,5}$/.test(fromType)) {
    return fromType.toLowerCase();
  }
  return 'png';
}

function sanitizeBaseName(file: File): string {
  const raw = file.name?.split('.').slice(0, -1).join('.') ?? '';
  const trimmed = raw.trim();
  if (!trimmed) {
    return 'image';
  }
  const normalized = trimmed.replace(/[^a-zA-Z0-9-_]+/g, '-').replace(/-{2,}/g, '-');
  const cleaned = normalized.replace(/^-+|-+$/g, '');
  return cleaned || 'image';
}

function createId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `att-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function collapseWhitespaceAroundPlaceholder(content: string, placeholder: string): string {
  if (!content.includes(placeholder)) {
    return content;
  }
  const index = content.indexOf(placeholder);
  const before = content.slice(0, index).replace(/[ \t]+$/g, ' ');
  const after = content.slice(index + placeholder.length).replace(/^[ \t]+/g, ' ');
  return `${before}${after}`
    .replace(/\s{3,}/g, ' ')
    .replace(/\n{3,}/g, '\n\n')
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
  const [task, setTask] = useState('');
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const t = useTranslation();
  const resolvedPlaceholder = placeholder ?? t('console.input.placeholder.idle');

  const translateWithFallback = useCallback(
    (key: string, params: Record<string, unknown> | undefined, fallback: string): string => {
      try {
        const value = params ? t(key as any, params as any) : t(key as any);
        if (typeof value !== 'string' || value === key) {
          return fallback;
        }
        return value;
      } catch (error) {
        console.warn('[TaskInput] Missing translation', { key, error });
        return fallback;
      }
    },
    [t],
  );

  // Auto-resize textarea
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = `${textareaRef.current.scrollHeight}px`;
    }
  }, [task]);

  useEffect(() => {
    if (typeof prefill !== 'string') return;
    const nextValue = prefill.trim();
    if (!nextValue) return;

    setTask(prefill);

    const focusField = () => {
      if (!textareaRef.current) return;
      textareaRef.current.focus();
      const length = prefill.length;
      textareaRef.current.setSelectionRange(length, length);
    };

    if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
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
          const separator = surroundWithSpaces ? ' ' : '';
          return `${prev}${separator}${contentToInsert}`;
        });
        return;
      }

      const { selectionStart, selectionEnd, value } = textarea;
      const start = selectionStart ?? value.length;
      const end = selectionEnd ?? value.length;
      const before = value.slice(0, start);
      const after = value.slice(end);

      const needsPrefixSpace = surroundWithSpaces && before.length > 0 && !/\s$/.test(before);
      const needsSuffixSpace = surroundWithSpaces && after.length > 0 && !/^\s/.test(after);

      const prefix = needsPrefixSpace ? ' ' : '';
      const suffix = needsSuffixSpace ? ' ' : '';
      const nextValue = `${before}${prefix}${contentToInsert}${suffix}${after}`;
      const cursorPosition = before.length + prefix.length + contentToInsert.length;

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
        if (!file.type?.startsWith('image/')) {
          continue;
        }

        try {
          const dataUrl = await readFileAsDataURL(file);
          const base64 = dataUrl.split(',')[1];
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

          const pending: PendingAttachment = {
            id: createId(),
            name: candidate,
            mediaType: file.type || `image/${ext}`,
            data: base64,
            previewUrl: dataUrl,
            placeholder: `[${candidate}]`,
          };

          setAttachments((prev) => [...prev, pending]);
          insertPlaceholder(pending.placeholder);
        } catch (error) {
          console.error('[TaskInput] Failed to read attachment', error);
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
      event.target.value = '';
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
        if (item.kind === 'file') {
          const file = item.getAsFile();
          if (file && file.type.startsWith('image/')) {
            images.push(file);
          }
        }
      }

      if (!images.length) {
        return;
      }

      event.preventDefault();
      const text = event.clipboardData?.getData('text');
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
      setTask((prev) => collapseWhitespaceAroundPlaceholder(prev, target.placeholder));
    },
    [attachments],
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
        source: 'user_upload',
        description: attachment.description,
      }));
      onSubmit(task.trim(), uploads);
      setTask('');
      setAttachments([]);
    }
  };

  const isInputDisabled = disabled || loading || isRunning;
  const showStopButton = (loading || isRunning) && typeof onStop === 'function';
  const stopButtonDisabled = stopDisabled || stopPending;

  const attachLabel = translateWithFallback('task.input.attachImage', undefined, 'Attach image');
  const getRemoveLabel = useCallback(
    (name: string) => translateWithFallback('task.input.removeAttachment', { name }, `Remove attachment ${name}`),
    [translateWithFallback],
  );

  return (
    <form onSubmit={handleSubmit} className="w-full" data-testid="task-input-form">
      <div className="flex flex-col gap-2.5 sm:flex-row sm:items-end sm:gap-2.5">
        <div className="relative flex-1">
          <textarea
            ref={textareaRef}
            value={task}
            onChange={(e) => setTask(e.target.value)}
            onPaste={handlePaste}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSubmit(e);
              }
            }}
            placeholder={resolvedPlaceholder}
            disabled={isInputDisabled}
            rows={1}
            aria-label={t('task.input.ariaLabel')}
            data-testid="task-input"
            className="min-h-[2.75rem] max-h-32 w-full resize-none overflow-y-auto rounded-2xl border border-slate-300 bg-white/90 px-3.5 pr-12 py-2.5 text-[13px] text-slate-700 shadow-sm transition focus:border-slate-900 focus:outline-none focus:ring-2 focus:ring-slate-900/30 disabled:cursor-not-allowed disabled:opacity-60"
            style={{ fieldSizing: 'content' } as any}
          />
          <button
            type="button"
            onClick={openFilePicker}
            disabled={isInputDisabled}
            className={cn(
              'absolute right-2 top-2 inline-flex h-8 w-8 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-500 shadow-sm transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50',
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
            accept="image/*"
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
              'console-primary-action h-[2.75rem]',
              'bg-destructive text-destructive-foreground border-destructive hover:bg-destructive/90',
              'disabled:bg-destructive disabled:text-destructive-foreground',
            )}
            title={t('task.stop.title')}
            data-testid="task-stop"
          >
            {stopPending ? (
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 animate-pulse rounded-full bg-white/80" />
                {t('task.stop.pending')}
              </span>
            ) : (
              <span className="flex items-center gap-1.5">
                <Square className="h-3.5 w-3.5" />
                {t('task.stop.label')}
              </span>
            )}
          </button>
        ) : (
          <button
            type="submit"
            disabled={isInputDisabled || !task.trim()}
            className="console-primary-action h-[2.75rem]"
            title={loading ? t('task.submit.title.running') : t('task.submit.title.default')}
            data-testid="task-submit"
          >
            {loading ? (
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 animate-pulse rounded-full bg-white/80" />
                {t('task.submit.running')}
              </span>
            ) : (
              <span className="flex items-center gap-1.5">
                <Send className="h-3.5 w-3.5" />
                {t('task.submit.label')}
              </span>
            )}
          </button>
        )}
      </div>

      {attachments.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-3" data-testid="task-attachments">
          {attachments.map((attachment) => (
            <div
              key={attachment.id}
              className="relative h-24 w-24 overflow-hidden rounded-lg border border-slate-200 bg-slate-50 shadow-sm"
            >
              <Image
                src={attachment.previewUrl}
                alt={attachment.name}
                fill
                className="object-cover"
                sizes="96px"
                unoptimized
              />
              <button
                type="button"
                onClick={() => handleRemoveAttachment(attachment.id)}
                className="absolute right-1 top-1 inline-flex h-5 w-5 items-center justify-center rounded-full bg-black/50 text-white transition hover:bg-black"
                aria-label={getRemoveLabel(attachment.name)}
              >
                <X className="h-3 w-3" />
              </button>
              <div className="absolute inset-x-0 bottom-0 bg-black/60 px-1 py-0.5 text-[10px] font-medium text-white">
                <span className="line-clamp-2 break-words">{attachment.name}</span>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="mt-1 flex justify-end text-[10px] font-medium uppercase tracking-[0.35em] text-slate-300">
        {t('console.input.hotkeyHint')}
      </div>
    </form>
  );
}
