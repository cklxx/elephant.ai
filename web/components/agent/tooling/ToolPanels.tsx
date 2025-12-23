'use client';

import { ReactNode, useCallback, useState } from 'react';
import { Clipboard, ClipboardCheck } from 'lucide-react';
import { AttachmentPayload } from '@/lib/types';
import { parseContentSegments, buildAttachmentUri } from '@/lib/attachments';
import { ImagePreview } from '@/components/ui/image-preview';
import { VideoPreview } from '@/components/ui/video-preview';
import { ArtifactPreviewCard } from '../ArtifactPreviewCard';
import { userFacingToolResultText } from '@/lib/toolPresentation';

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
      className="inline-flex items-center gap-2 rounded-full border border-border bg-card px-3 py-1 text-[10px] font-semibold text-foreground transition hover:bg-muted"
      aria-label={copied ? successLabel : label}
    >
      {copied ? <ClipboardCheck className="h-3 w-3" /> : <Clipboard className="h-3 w-3" />}
      <span>{copied ? successLabel : label}</span>
    </button>
  );
}

export function SimplePanel({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2 rounded-xl border border-border bg-card/90 p-3 text-[11px] text-foreground/80">
      {children}
    </div>
  );
}

export function PanelHeader({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <p className="text-[11px] font-semibold text-muted-foreground">{title}</p>
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
      <pre className="max-h-56 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-background px-3 py-2 font-mono text-[11px] leading-snug text-foreground/80">
        {args}
      </pre>
    </SimplePanel>
  );
}

export function ToolResultPanel({
  toolName,
  result,
  error,
  resultTitle,
  errorTitle,
  copyLabel,
  copyErrorLabel,
  copiedLabel,
  attachments,
  metadata,
}: {
  toolName?: string | null;
  result: any;
  error?: string | null;
  resultTitle: string;
  errorTitle: string;
  copyLabel: string;
  copyErrorLabel: string;
  copiedLabel: string;
  attachments?: Record<string, AttachmentPayload>;
  metadata?: Record<string, any> | null;
}) {
  if (error) {
    return (
      <SimplePanel>
        <PanelHeader title={errorTitle} action={<CopyButton label={copyErrorLabel} successLabel={copiedLabel} value={error} />} />
        <p className="text-sm font-semibold text-destructive">{error}</p>
      </SimplePanel>
    );
  }

  const rawText =
    typeof result === 'string'
      ? result
      : result
        ? JSON.stringify(result, null, 2)
        : '';
  const formatted = userFacingToolResultText({
    toolName,
    result: rawText,
    metadata,
    attachments: attachments ?? null,
  });

  const attachmentsAvailable = attachments && Object.keys(attachments).length > 0;
  const segments = attachmentsAvailable
    ? parseContentSegments(formatted || '', attachments)
    : null;
  const textSegments = segments
    ? segments.filter((segment) => segment.type === 'text' && segment.text && segment.text.length > 0)
    : [];
  const mediaSegments = segments
    ? segments.filter(
        (segment) =>
          (segment.type === 'image' || segment.type === 'video') &&
          segment.attachment,
      )
    : [];
  const artifactSegments = segments
    ? segments.filter(
        (segment) =>
          (segment.type === 'document' || segment.type === 'embed') &&
          segment.attachment,
      )
    : [];

  if (!formatted && !attachmentsAvailable) {
    return null;
  }

  return (
    <SimplePanel>
      <PanelHeader title={resultTitle} action={<CopyButton label={copyLabel} successLabel={copiedLabel} value={formatted} />} />
      {attachmentsAvailable ? (
        <div className="rounded-lg border border-border/60 bg-background p-3">
          {(textSegments.length > 0 || formatted.trim().length > 0) && (
            <div className="max-h-56 overflow-auto whitespace-pre-wrap text-[11px] leading-normal text-foreground/80">
              {textSegments.length > 0
                ? textSegments.map((segment, index) => (
                    <span key={`tool-result-text-${index}`}>{segment.text}</span>
                  ))
                : formatted}
            </div>
          )}
          {mediaSegments.length > 0 && (
            <div className="mt-4 grid gap-4 sm:grid-cols-2">
              {mediaSegments.map((segment, index) => {
                if (!segment.attachment) {
                  return null;
                }
                const uri = buildAttachmentUri(segment.attachment);
                if (!uri) {
                  return null;
                }
                const key = segment.placeholder || `${segment.type}-${index}`;
                if (segment.type === 'video') {
                  return (
                    <VideoPreview
                      key={`tool-result-media-${key}`}
                      src={uri}
                      mimeType={segment.attachment.media_type || 'video/mp4'}
                      description={segment.attachment.description}
                      maxHeight="16rem"
                    />
                  );
                }
                return (
                  <ImagePreview
                    key={`tool-result-media-${key}`}
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
          {artifactSegments.length > 0 && (
            <div className="mt-4 space-y-3">
              {artifactSegments.map((segment, index) => {
                if (!segment.attachment) {
                  return null;
                }
                const key = segment.placeholder || `artifact-${index}`;
                return (
                  <ArtifactPreviewCard
                    key={`tool-panel-artifact-${key}`}
                    attachment={segment.attachment}
                  />
                );
              })}
            </div>
          )}
        </div>
      ) : (
        <pre className="max-h-56 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-background px-3 py-2 text-[11px] leading-snug text-foreground/80">
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
      <pre className="max-h-48 overflow-auto whitespace-pre-wrap font-mono text-[10px] leading-snug text-muted-foreground">
        {content.trim()}
      </pre>
    </SimplePanel>
  );
}
