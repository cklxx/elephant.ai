'use client';

import type { KeyboardEvent, MouseEvent } from "react";
import { useEffect, useMemo, useState } from 'react';
import Image from 'next/image';
import { AttachmentPayload } from '@/lib/types';
import { cn } from '@/lib/utils';
import { resolveAttachmentDownloadUris } from '@/lib/attachments';
import { LazyMarkdownRenderer } from "@/components/agent/LazyMarkdownRenderer";
import { Download, ExternalLink, FileText, FileCode, Loader2, X } from "lucide-react";
import { Dialog, DialogClose, DialogContent, DialogTitle } from "@/components/ui/dialog";

const AUTO_PREFETCH_MARKDOWN_MAX_BYTES = 512 * 1024;
const MARKDOWN_SNIPPET_MAX_CHARS = 1400;

interface ArtifactPreviewCardProps {
  attachment: AttachmentPayload;
  className?: string;
}

export function ArtifactPreviewCard({
  attachment,
  className,
}: ArtifactPreviewCardProps) {
  const [isPreviewOpen, setIsPreviewOpen] = useState(false);
  const [prefetchRequested, setPrefetchRequested] = useState(false);
  const [markdownPreview, setMarkdownPreview] = useState<string | null>(null);
  const [markdownLoading, setMarkdownLoading] = useState(false);

  const { preferredUri: downloadUri, fallbackUri: originalUri, preferredKind } =
    resolveAttachmentDownloadUris(attachment);
  const preferredDownloadName =
    preferredKind === "pdf" && attachment.name
      ? `${attachment.name.replace(/\.[^.]+$/, "")}.pdf`
      : attachment.name;
  const fallbackLabel =
    preferredKind === "pdf" && originalUri
      ? `Download ${attachment.format?.toUpperCase() || "PPTX"}`
      : null;
  const primaryDownloadLabel = preferredKind === "pdf" ? "Download PDF" : "Download";
  const displayName = attachment.description || attachment.name || "Artifact";
  const formatLabel = attachment.format?.toUpperCase() || attachment.media_type || "FILE";
  const isMarkdown = formatLabel.includes("MARKDOWN") || attachment.media_type?.includes("markdown");

  // Decide Icon
  const FileIcon = isMarkdown ? FileText : FileCode;

  const previewAssets = attachment.preview_assets || [];
  const imageAssets = previewAssets.filter((asset) => asset.mime_type?.startsWith("image/"));
  const htmlAsset = previewAssets.find((asset) => asset.mime_type?.includes("html"));
  const previewImageUrl =
    imageAssets.find((asset) => typeof asset.cdn_url === "string" && asset.cdn_url.trim())?.cdn_url ??
    null;

  const canInlinePreview = Boolean(htmlAsset) || isMarkdown;

  const markdownSnippet = useMemo(() => {
    if (!markdownPreview) return null;
    const trimmed = markdownPreview.trim();
    if (!trimmed) return null;
    if (trimmed.length <= MARKDOWN_SNIPPET_MAX_CHARS) return trimmed;
    return `${trimmed
      .slice(0, MARKDOWN_SNIPPET_MAX_CHARS)
      .replace(/\s+$/, "")}\n…`;
  }, [markdownPreview]);

  const requestPreview = () => {
    if (!canInlinePreview) return;
    if (isMarkdown) setPrefetchRequested(true);
  };

  const openPreview = () => {
    if (!canInlinePreview) return;
    requestPreview();
    setIsPreviewOpen(true);
  };

  const stopPropagation = (event: MouseEvent) => {
    event.stopPropagation();
  };

  const onCardKeyDown = (event: KeyboardEvent) => {
    if (!canInlinePreview) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openPreview();
    }
  };

  useEffect(() => {
    if (!isMarkdown) return;
    if (prefetchRequested) return;

    const size = typeof attachment.size === "number" ? attachment.size : null;
    if (size && size > AUTO_PREFETCH_MARKDOWN_MAX_BYTES) {
      return;
    }

    setPrefetchRequested(true);
  }, [attachment.size, isMarkdown, prefetchRequested]);

  // Effect to load markdown (for snippet + modal).
  useEffect(() => {
    if (!isMarkdown || !downloadUri) return;
    if (!prefetchRequested && !isPreviewOpen) return;
    if (markdownPreview !== null) return;

    let cancelled = false;
    setMarkdownLoading(true);

    async function fetchMd() {
      try {
        const res = await fetch(downloadUri);
        const text = await res.text();
        if (!cancelled) setMarkdownPreview(text);
      } catch (e) {
        console.error("Failed to load md", e);
      } finally {
        if (!cancelled) setMarkdownLoading(false);
      }
    }
    fetchMd();

    return () => { cancelled = true; };
  }, [downloadUri, isMarkdown, prefetchRequested, isPreviewOpen, markdownPreview]);

  return (
    <>
      <div
        className={cn(
          "group relative w-full overflow-hidden rounded-xl border border-border/40 bg-card transition-colors",
          canInlinePreview && "cursor-pointer hover:bg-muted/10",
          className,
        )}
        role={canInlinePreview ? "button" : undefined}
        tabIndex={canInlinePreview ? 0 : undefined}
        aria-label={canInlinePreview ? `Preview ${displayName}` : undefined}
        onClick={canInlinePreview ? openPreview : undefined}
        onKeyDown={canInlinePreview ? onCardKeyDown : undefined}
        onMouseEnter={canInlinePreview ? requestPreview : undefined}
        onFocus={canInlinePreview ? requestPreview : undefined}
      >
        {/* Header / Main Body - Manus Style File Card */}
        <div className="flex items-center gap-3 p-4">
          {/* Icon Box */}
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-400">
            {previewImageUrl ? (
              <Image
                src={previewImageUrl}
                alt=""
                width={40}
                height={40}
                className="h-full w-full rounded-lg object-cover"
                unoptimized
                sizes="40px"
              />
            ) : (
              <FileIcon className="h-5 w-5" />
            )}
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0 grid gap-0.5">
            <h4 className="text-sm font-medium text-foreground truncate">
              {displayName}
            </h4>
            <div className="flex items-center gap-2 text-xs text-muted-foreground/80">
              <span>{formatLabel}</span>
              {attachment.size && (
                <>
                  <span>•</span>
                  <span>{Math.round(attachment.size / 1024)} KB</span>
                </>
              )}
            </div>
          </div>

          {/* Actions - Minimal */}
          <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
            {downloadUri && (
              <>
                <a
                  href={downloadUri}
                  className="p-2 rounded-md hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
                  title={preferredKind === "pdf" ? "Open PDF" : "View"}
                  aria-label={preferredKind === "pdf" ? "Open PDF" : "View"}
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={stopPropagation}
                >
                  <ExternalLink className="w-4 h-4" />
                </a>
                <a
                  href={downloadUri}
                  download={preferredDownloadName}
                  className="p-2 rounded-md hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
                  title={primaryDownloadLabel}
                  aria-label={primaryDownloadLabel}
                  onClick={stopPropagation}
                >
                  <Download className="w-4 h-4" />
                </a>
                {originalUri ? (
                  <a
                    href={originalUri}
                    download={attachment.name}
                    className="p-2 rounded-md hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
                    title={fallbackLabel ?? "Download original"}
                    aria-label={fallbackLabel ?? "Download original"}
                    onClick={stopPropagation}
                  >
                    <Download className="w-4 h-4" />
                  </a>
                ) : null}
              </>
            )}
          </div>
        </div>

        {/* Inline excerpt (markdown only) */}
        {isMarkdown ? (
          <div className="border-t border-border/40 px-4 py-3">
            {markdownLoading && !markdownPreview ? (
              <div className="h-24 w-full animate-pulse rounded-lg border border-border/40 bg-muted/20" />
            ) : markdownSnippet ? (
              <div className="relative max-h-64 overflow-hidden pointer-events-none">
                <LazyMarkdownRenderer
                  content={markdownSnippet}
                  containerClassName="markdown-body"
                  className={cn(
                    "prose prose-sm max-w-none text-foreground/90",
                    "prose-p:my-1.5 prose-headings:my-1 prose-li:my-0.5 prose-pre:my-2",
                  )}
                />
                <div className="pointer-events-none absolute inset-x-0 bottom-0 h-12 bg-gradient-to-t from-card via-card/80 to-transparent" />
              </div>
            ) : (
              <div className="text-xs text-muted-foreground">Click to preview</div>
            )}
          </div>
        ) : null}
      </div>

      {canInlinePreview && (
        <Dialog open={isPreviewOpen} onOpenChange={setIsPreviewOpen}>
          <DialogContent
            className="w-[90vw] h-[90vh] max-w-[90vw] max-h-[90vh] p-0 overflow-hidden rounded-2xl border border-border/70 bg-background"
            onClose={() => setIsPreviewOpen(false)}
            showCloseButton={false}
          >
            <DialogTitle className="sr-only">{displayName}</DialogTitle>
            <div className="flex h-full flex-col">
              <div className="flex items-center justify-between gap-4 border-b border-border/60 px-6 py-4">
                <div className="min-w-0 flex items-center gap-3">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-400">
                    <FileIcon className="h-5 w-5" />
                  </div>
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold text-foreground">
                      {displayName}
                    </div>
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                      <span>{formatLabel}</span>
                      {attachment.size ? (
                        <>
                          <span>•</span>
                          <span>{Math.round(attachment.size / 1024)} KB</span>
                        </>
                      ) : null}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-1">
                  {downloadUri ? (
                    <>
                      <a
                        href={downloadUri}
                        className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                        title={preferredKind === "pdf" ? "Open PDF in new tab" : "Open in new tab"}
                        aria-label={preferredKind === "pdf" ? "Open PDF in new tab" : "Open in new tab"}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        <ExternalLink className="h-4 w-4" />
                      </a>
                      <a
                        href={downloadUri}
                        download={preferredDownloadName}
                        className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                        title={primaryDownloadLabel}
                        aria-label={primaryDownloadLabel}
                      >
                        <Download className="h-4 w-4" />
                      </a>
                      {originalUri ? (
                        <a
                          href={originalUri}
                          download={attachment.name}
                          className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                          title={fallbackLabel ?? "Download original"}
                          aria-label={fallbackLabel ?? "Download original"}
                        >
                          <Download className="h-4 w-4" />
                        </a>
                      ) : null}
                    </>
                  ) : null}
                  <DialogClose asChild>
                    <button
                      type="button"
                      className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                      aria-label="Close preview"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  </DialogClose>
                </div>
              </div>

              <div className="flex-1 overflow-auto bg-muted/30 px-6 py-6">
                {isMarkdown ? (
                  markdownLoading && !markdownPreview ? (
                    <div className="flex items-center justify-center py-10 text-sm text-muted-foreground gap-2">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Loading preview…
                    </div>
                  ) : markdownPreview ? (
                    <div className="mx-auto w-full max-w-[820px] rounded-xl border border-border/60 bg-background p-8 shadow-sm">
                      <LazyMarkdownRenderer
                        content={markdownPreview}
                        containerClassName="markdown-body"
                        className="prose max-w-none text-base leading-normal text-foreground"
                      />
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">
                      Preview unavailable.
                    </div>
                  )
                ) : htmlAsset ? (
                  <div className="mx-auto w-full max-w-[980px]">
                    <iframe
                      src={htmlAsset.cdn_url}
                      className="h-[80vh] w-full rounded-xl border border-border/60 bg-white"
                      title="Preview"
                    />
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">
                    Preview unavailable.
                  </div>
                )}
              </div>
            </div>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
