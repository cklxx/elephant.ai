'use client';

import { ReactNode, useCallback, useState } from 'react';
import { Clipboard, ClipboardCheck } from 'lucide-react';
import { AttachmentPayload } from '@/lib/types';
import { parseContentSegments, buildAttachmentUri } from '@/lib/attachments';
import { ImagePreview } from '@/components/ui/image-preview';

function fallbackCopy(text: string) {
  try {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'absolute';
    textarea.style.left = '-9999px';
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    document.body.removeChild(textarea);
  } catch (error) {
    console.error('Failed to copy tool output', error);
  }
}

export function CopyButton({
  label,
  successLabel,
  value,
}: {
  label: string;
  successLabel: string;
  value?: string | null;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    if (!value) return;

    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(value);
      } else {
        fallbackCopy(value);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch (error) {
      fallbackCopy(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    }
  }, [value]);

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="inline-flex items-center gap-2 rounded-full border border-border bg-card px-3 py-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-foreground transition hover:-translate-y-0.5 hover:-translate-x-0.5 hover:shadow-[6px_6px_0_rgba(0,0,0,0.55)]"
      aria-label={copied ? successLabel : label}
    >
      {copied ? <ClipboardCheck className="h-3 w-3" /> : <Clipboard className="h-3 w-3" />}
      <span>{copied ? successLabel : label}</span>
    </button>
  );
}

export function SimplePanel({ children }: { children: ReactNode }) {
  return (
    <div className="space-y-2 rounded-xl border-2 border-border bg-card/90 p-3 text-[11px] text-foreground/80 shadow-[6px_6px_0_rgba(0,0,0,0.55)]">
      {children}
    </div>
  );
}

export function PanelHeader({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <p className="console-microcopy font-semibold uppercase tracking-[0.3em] text-muted-foreground">{title}</p>
      {action}
    </div>
  );
}

export function ToolArgumentsPanel({
  args,
  label,
  copyLabel,
  copiedLabel,
}: {
  args: string;
  label: string;
  copyLabel: string;
  copiedLabel: string;
}) {
  return (
    <SimplePanel>
      <PanelHeader title={label} action={<CopyButton label={copyLabel} successLabel={copiedLabel} value={args} />} />
      <pre className="console-scrollbar max-h-56 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-background px-3 py-2 font-mono text-[11px] leading-relaxed text-foreground/80">
        {args}
      </pre>
    </SimplePanel>
  );
}

export function ToolResultPanel({
  result,
  error,
  resultTitle,
  errorTitle,
  copyLabel,
  copyErrorLabel,
  copiedLabel,
  attachments,
}: {
  result: any;
  error?: string | null;
  resultTitle: string;
  errorTitle: string;
  copyLabel: string;
  copyErrorLabel: string;
  copiedLabel: string;
  attachments?: Record<string, AttachmentPayload>;
}) {
  if (error) {
    return (
      <SimplePanel>
        <PanelHeader title={errorTitle} action={<CopyButton label={copyErrorLabel} successLabel={copiedLabel} value={error} />} />
        <p className="console-microcopy font-semibold text-destructive">{error}</p>
      </SimplePanel>
    );
  }

  const formatted =
    typeof result === 'string'
      ? result
      : result
        ? JSON.stringify(result, null, 2)
        : '';

  const attachmentsAvailable = attachments && Object.keys(attachments).length > 0;
  const segments = attachmentsAvailable
    ? parseContentSegments(formatted || '', attachments)
    : null;
  const textSegments = segments
    ? segments.filter((segment) => segment.type === 'text' && segment.text && segment.text.length > 0)
    : [];
  const imageSegments = segments
    ? segments.filter((segment) => segment.type === 'image' && segment.attachment)
    : [];

  if (!formatted && !attachmentsAvailable) {
    return null;
  }

  return (
    <SimplePanel>
      <PanelHeader title={resultTitle} action={<CopyButton label={copyLabel} successLabel={copiedLabel} value={formatted} />} />
      {attachmentsAvailable ? (
        <div className="rounded-lg border border-border/60 bg-background p-3">
          <div className="console-scrollbar max-h-56 overflow-auto whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-foreground/80">
            {textSegments.length > 0
              ? textSegments.map((segment, index) => (
                  <span key={`tool-result-text-${index}`}>{segment.text}</span>
                ))
              : formatted}
          </div>
          {imageSegments.length > 0 && (
            <div className="mt-4 grid gap-4 sm:grid-cols-2">
              {imageSegments.map((segment, index) => {
                if (!segment.attachment) {
                  return null;
                }
                const uri = buildAttachmentUri(segment.attachment);
                if (!uri) {
                  return null;
                }
                return (
                  <ImagePreview
                    key={`tool-result-image-${index}`}
                    src={uri}
                    alt={segment.attachment.description || segment.attachment.name}
                    minHeight="10rem"
                    maxHeight="16rem"
                    sizes="(min-width: 1280px) 25vw, (min-width: 768px) 40vw, 100vw"
                  />
                );
              })}
            </div>
          )}
        </div>
      ) : (
        <pre className="console-scrollbar max-h-56 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-background px-3 py-2 font-mono text-[11px] leading-relaxed text-foreground/80">
          {formatted}
        </pre>
      )}
    </SimplePanel>
  );
}

export function ToolStreamPanel({ title, content }: { title: string; content: string }) {
  return (
    <SimplePanel>
      <PanelHeader title={title} />
      <pre className="console-scrollbar max-h-48 overflow-auto whitespace-pre-wrap font-mono text-[10px] leading-snug text-slate-600">
        {content.trim()}
      </pre>
    </SimplePanel>
  );
}
