"use client";

import type { KeyboardEvent, MouseEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import { AttachmentPayload } from "@/lib/types";
import { cn } from "@/lib/utils";
import { resolveAttachmentDownloadUris } from "@/lib/attachments";
import { LazyMarkdownRenderer } from "@/components/agent/LazyMarkdownRenderer";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Download,
  ExternalLink,
  FileText,
  FileCode,
  Loader2,
  X,
} from "lucide-react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";

const AUTO_PREFETCH_MARKDOWN_MAX_BYTES = 512 * 1024;
const MARKDOWN_SNIPPET_MAX_CHARS = 1400;
const HTML_EDITOR_MIN_HEIGHT = "min-h-[360px]";

type HtmlValidationIssue = {
  level: "error" | "warning";
  message: string;
};

function decodeBase64ToText(value: string): string {
  const binary = atob(value);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function decodeHtmlFromDataUri(uri: string): string | null {
  if (!uri.startsWith("data:")) return null;
  const commaIndex = uri.indexOf(",");
  if (commaIndex === -1) return null;
  const meta = uri.slice(5, commaIndex);
  const data = uri.slice(commaIndex + 1);
  if (/;base64/i.test(meta)) {
    try {
      return decodeBase64ToText(data);
    } catch (error) {
      console.error("Failed to decode base64 HTML", error);
      return null;
    }
  }
  try {
    return decodeURIComponent(data);
  } catch (error) {
    console.error("Failed to decode HTML data URI", error);
    return null;
  }
}

async function loadHtmlSource(uri: string): Promise<string> {
  const decoded = decodeHtmlFromDataUri(uri);
  if (decoded !== null) return decoded;
  const response = await fetch(uri);
  if (!response.ok) {
    throw new Error(`Failed to load HTML (${response.status})`);
  }
  return response.text();
}

function ensureViewportMeta(html: string): string {
  const trimmed = html.trim();
  if (!trimmed) return html;
  const lower = trimmed.toLowerCase();
  if (lower.includes('name="viewport"') || lower.includes("name='viewport'")) {
    return html;
  }

  const viewportTag =
    '<meta name="viewport" content="width=device-width, initial-scale=1" />';
  if (/<head\b[^>]*>/i.test(trimmed)) {
    return trimmed.replace(
      /<head\b[^>]*>/i,
      (match) => `${match}\n  ${viewportTag}`,
    );
  }
  if (/<body\b[^>]*>/i.test(trimmed)) {
    return trimmed.replace(
      /<body\b[^>]*>/i,
      (match) => `<head>\n  ${viewportTag}\n</head>\n${match}`,
    );
  }
  if (/<html\b[^>]*>/i.test(trimmed)) {
    return trimmed.replace(
      /<html\b[^>]*>/i,
      (match) => `${match}\n<head>\n  ${viewportTag}\n</head>`,
    );
  }

  return `<!doctype html><html><head>${viewportTag}</head><body>${trimmed}</body></html>`;
}

function buildHtmlDataUri(html: string): string {
  const normalized = ensureViewportMeta(html);
  return `data:text/html;charset=utf-8,${encodeURIComponent(normalized)}`;
}

function validateHtmlSource(html: string): HtmlValidationIssue[] {
  const trimmed = html.trim();
  if (!trimmed) {
    return [{ level: "error", message: "HTML is empty." }];
  }

  const lower = trimmed.toLowerCase();
  const issues: HtmlValidationIssue[] = [];
  const htmlTags = (lower.match(/<html\b/g) ?? []).length;
  const bodyTags = (lower.match(/<body\b/g) ?? []).length;
  const headTags = (lower.match(/<head\b/g) ?? []).length;

  if (!lower.includes("<!doctype html")) {
    issues.push({ level: "warning", message: "Missing <!DOCTYPE html>." });
  }
  if (htmlTags === 0) {
    issues.push({ level: "warning", message: "Missing <html> tag." });
  } else if (htmlTags > 1) {
    issues.push({ level: "error", message: "Multiple <html> tags found." });
  }
  if (headTags === 0) {
    issues.push({ level: "warning", message: "Missing <head> tag." });
  } else if (headTags > 1) {
    issues.push({ level: "warning", message: "Multiple <head> tags found." });
  }
  if (bodyTags === 0) {
    issues.push({ level: "warning", message: "Missing <body> tag." });
  } else if (bodyTags > 1) {
    issues.push({ level: "warning", message: "Multiple <body> tags found." });
  }
  if (!lower.includes("<meta charset")) {
    issues.push({ level: "warning", message: "Missing <meta charset>." });
  }
  if (!/name=["']viewport["']/.test(lower)) {
    issues.push({
      level: "warning",
      message: 'Missing <meta name="viewport">.',
    });
  }
  if (!lower.includes("<title")) {
    issues.push({ level: "warning", message: "Missing <title> tag." });
  }

  const openScripts = (lower.match(/<script\b/g) ?? []).length;
  const closeScripts = (lower.match(/<\/script>/g) ?? []).length;
  if (openScripts !== closeScripts) {
    issues.push({ level: "error", message: "Mismatched <script> tags." });
  }

  const openStyles = (lower.match(/<style\b/g) ?? []).length;
  const closeStyles = (lower.match(/<\/style>/g) ?? []).length;
  if (openStyles !== closeStyles) {
    issues.push({ level: "warning", message: "Mismatched <style> tags." });
  }

  return issues;
}

interface ArtifactPreviewCardProps {
  attachment: AttachmentPayload;
  className?: string;
  compact?: boolean;
  showInlinePreview?: boolean;
}

const normalizeTitle = (value: string | null) =>
  value
    ?.replace(/^\uFEFF/, "")
    .replace(/\.[^.]+$/, "")
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, " ")
    .trim() || null;

const stripRedundantHeading = (
  markdown: string,
  normalizedTitle: string | null,
) => {
  if (!markdown.trim()) return markdown;

  const lines = markdown.replace(/^\uFEFF/, "").split(/\r?\n/);

  let index = 0;
  while (index < lines.length && !lines[index].trim()) {
    index += 1;
  }

  if (index >= lines.length) {
    return markdown.trimStart();
  }

  const headingMatch = lines[index].match(/^#{1,6}\s+(.+?)\s*#*\s*$/);
  const headingText = headingMatch ? normalizeTitle(headingMatch[1]) : null;

  if (headingText && normalizedTitle && headingText === normalizedTitle) {
    index += 1;
    while (index < lines.length && !lines[index].trim()) {
      index += 1;
    }
    return lines.slice(index).join("\n").trimStart();
  }

  return markdown.trimStart();
};

export function ArtifactPreviewCard({
  attachment,
  className,
  compact = false,
  showInlinePreview = true,
}: ArtifactPreviewCardProps) {
  const [isPreviewOpen, setIsPreviewOpen] = useState(false);
  const [prefetchRequested, setPrefetchRequested] = useState(false);
  const [markdownPreview, setMarkdownPreview] = useState<string | null>(null);
  const [markdownLoading, setMarkdownLoading] = useState(false);
  const [htmlPrefetchRequested, setHtmlPrefetchRequested] = useState(false);
  const [htmlSource, setHtmlSource] = useState<string | null>(null);
  const [htmlDraft, setHtmlDraft] = useState<string | null>(null);
  const [htmlLoading, setHtmlLoading] = useState(false);
  const [htmlError, setHtmlError] = useState<string | null>(null);
  const [htmlView, setHtmlView] = useState<"preview" | "source">("preview");

  const {
    preferredUri: downloadUri,
    fallbackUri: originalUri,
    preferredKind,
  } = resolveAttachmentDownloadUris(attachment);
  const preferredDownloadName =
    preferredKind === "pdf" && attachment.name
      ? `${attachment.name.replace(/\.[^.]+$/, "")}.pdf`
      : attachment.name;
  const fallbackLabel =
    preferredKind === "pdf" && originalUri
      ? `Download ${attachment.format?.toUpperCase() || "PPTX"}`
      : null;
  const primaryDownloadLabel =
    preferredKind === "pdf" ? "Download PDF" : "Download";
  const primaryTitle = attachment.description || attachment.name;
  const displayName = primaryTitle || "Artifact";
  const formatLabel =
    attachment.format?.toUpperCase() || attachment.media_type || "FILE";
  const isMarkdown =
    formatLabel.includes("MARKDOWN") ||
    attachment.media_type?.includes("markdown");
  const normalizedTitle = normalizeTitle(primaryTitle ?? null);

  // Decide Icon
  const FileIcon = isMarkdown ? FileText : FileCode;

  const previewAssets = attachment.preview_assets || [];
  const imageAssets = previewAssets.filter((asset) =>
    asset.mime_type?.startsWith("image/"),
  );
  const isHTML =
    attachment.media_type?.toLowerCase().includes("html") ||
    attachment.format?.toLowerCase() === "html" ||
    attachment.preview_profile?.toLowerCase().includes("document.html");
  const htmlAsset =
    previewAssets.find((asset) => asset.mime_type?.includes("html")) ??
    (isHTML && attachment.uri
      ? {
          asset_id: `${attachment.name || "html"}-preview`,
          label: "HTML preview",
          mime_type: attachment.media_type || "text/html",
          cdn_url: attachment.uri,
          preview_type: "iframe",
        }
      : undefined);
  const htmlSourceUri = htmlAsset?.cdn_url || downloadUri || originalUri;
  const previewImageUrl =
    imageAssets.find(
      (asset) => typeof asset.cdn_url === "string" && asset.cdn_url.trim(),
    )?.cdn_url ?? null;

  const canInlinePreview = Boolean(htmlAsset) || isMarkdown;

  const normalizedMarkdown = useMemo(() => {
    if (!markdownPreview) return null;
    const trimmed = markdownPreview.replace(/^\uFEFF/, "").trim();
    return trimmed || null;
  }, [markdownPreview]);

  const markdownSnippet = useMemo(() => {
    if (!normalizedMarkdown) return null;
    const withoutHeading = stripRedundantHeading(
      normalizedMarkdown,
      normalizedTitle,
    );
    const trimmed = withoutHeading.trim();
    if (!trimmed) return null;
    if (trimmed.length <= MARKDOWN_SNIPPET_MAX_CHARS) return trimmed;
    return `${trimmed
      .slice(0, MARKDOWN_SNIPPET_MAX_CHARS)
      .replace(/\s+$/, "")}\n…`;
  }, [normalizedMarkdown, normalizedTitle]);

  const requestPreview = () => {
    if (!canInlinePreview) return;
    if (isMarkdown) setPrefetchRequested(true);
    if (isHTML) setHtmlPrefetchRequested(true);
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
    const uri = downloadUri;
    if (!isMarkdown || !uri) return;
    const markdownUri: string = uri;
    if (!prefetchRequested && !isPreviewOpen) return;
    if (markdownPreview !== null) return;

    let cancelled = false;
    setMarkdownLoading(true);

    async function fetchMd() {
      try {
        const res = await fetch(markdownUri);
        const text = await res.text();
        if (!cancelled) setMarkdownPreview(text);
      } catch (e) {
        console.error("Failed to load md", e);
      } finally {
        if (!cancelled) setMarkdownLoading(false);
      }
    }
    fetchMd();

    return () => {
      cancelled = true;
    };
  }, [
    downloadUri,
    isMarkdown,
    prefetchRequested,
    isPreviewOpen,
    markdownPreview,
  ]);

  useEffect(() => {
    if (!isHTML) return;
    if (!htmlSourceUri) return;
    if (!htmlPrefetchRequested && !isPreviewOpen) return;
    if (htmlSource !== null || htmlLoading) return;

    let cancelled = false;
    setHtmlLoading(true);
    setHtmlError(null);

    loadHtmlSource(htmlSourceUri)
      .then((html) => {
        if (cancelled) return;
        setHtmlSource(html);
        setHtmlDraft((current) => current ?? html);
      })
      .catch((error) => {
        if (cancelled) return;
        setHtmlError(
          error instanceof Error ? error.message : "Failed to load HTML.",
        );
      })
      .finally(() => {
        if (!cancelled) setHtmlLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [
    htmlSourceUri,
    htmlPrefetchRequested,
    isPreviewOpen,
    isHTML,
    htmlSource,
    htmlLoading,
  ]);

  const htmlBaseline = htmlSource ?? "";
  const htmlContent = htmlDraft ?? htmlBaseline;
  const htmlReady = Boolean(htmlSource) || htmlDraft !== null;
  const htmlIssues = useMemo(
    () => (htmlReady ? validateHtmlSource(htmlContent) : []),
    [htmlReady, htmlContent],
  );
  const htmlErrors = htmlIssues.filter((issue) => issue.level === "error");
  const htmlWarnings = htmlIssues.filter((issue) => issue.level === "warning");
  const htmlStatus =
    htmlErrors.length > 0
      ? "error"
      : htmlWarnings.length > 0
        ? "warning"
        : "ok";
  const htmlPreviewUri = useMemo(() => {
    if (htmlContent.trim().length > 0) {
      return buildHtmlDataUri(htmlContent);
    }
    return htmlAsset?.cdn_url ?? null;
  }, [htmlContent, htmlAsset?.cdn_url]);
  const isHtmlDirty = htmlDraft !== null && htmlDraft !== htmlBaseline;

  return (
    <>
      <div
        className={cn(
          "group relative w-full overflow-hidden rounded-xl border border-border/40 bg-card transition-colors",
          compact && "flex aspect-[1.618/1] max-w-[420px] flex-col",
          canInlinePreview &&
            "cursor-pointer hover:border-ring/60 hover:ring-1 hover:ring-ring/30 focus-visible:outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/60",
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
        <div
          className={cn(
            "flex items-center gap-3",
            compact ? "px-3 py-2" : "px-4 py-3",
          )}
        >
          {/* Icon Box */}
          <div
            className={cn(
              "flex shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-400",
              compact ? "h-8 w-8" : "h-10 w-10",
            )}
          >
            {previewImageUrl ? (
              <Image
                src={previewImageUrl}
                alt=""
                width={compact ? 32 : 40}
                height={compact ? 32 : 40}
                className="h-full w-full rounded-lg object-cover"
                unoptimized
                sizes={compact ? "32px" : "40px"}
              />
            ) : (
              <FileIcon className={compact ? "h-4 w-4" : "h-5 w-5"} />
            )}
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0 grid gap-0.5">
            <h4
              className="text-sm font-medium text-foreground truncate"
              style={{
                marginTop: "0px",
                marginBottom: "0px",
              }}
            >
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
        <div
          className={cn(
            "border-t border-border/40",
            compact && "flex-1 min-h-0",
            compact ? "px-3 py-2" : "px-4 py-2",
          )}
        >
            {showInlinePreview ? (
              markdownLoading && !markdownPreview ? (
                <div
                  className={cn(
                    "w-full animate-pulse rounded-lg border border-border/40 bg-muted/20",
                    compact ? "h-16" : "h-24",
                  )}
                />
              ) : markdownSnippet ? (
                <div
                  className={cn(
                    "relative overflow-hidden pointer-events-none",
                    compact ? "h-full" : "max-h-64",
                  )}
                >
                  <LazyMarkdownRenderer
                    content={markdownSnippet}
                    containerClassName="markdown-compact"
                    className={cn(
                      "flex flex-col",
                      "prose prose-sm max-w-none text-foreground/90",
                      compact ? "leading-snug" : "leading-normal",
                      "prose-p:my-1.5 prose-headings:my-1 prose-li:my-0.5 prose-pre:my-2",
                      "prose-ol:flex prose-ol:flex-col prose-ul:flex prose-ul:flex-col prose-li:flex prose-li:flex-col",
                    )}
                  />
                  <div
                    className={cn(
                      "pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-card via-card/80 to-transparent",
                      compact ? "h-10" : "h-12",
                    )}
                  />
                </div>
              ) : (
                <div className="text-xs text-muted-foreground">
                  Click to preview
                </div>
              )
            ) : (
              <div className="text-xs text-muted-foreground">
                Click to preview
              </div>
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
            <div className="flex h-full min-h-0 min-w-0 flex-col">
              <div className="flex items-center justify-between gap-4 border-b border-border/60 px-6 py-3">
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
                        title={
                          preferredKind === "pdf"
                            ? "Open PDF in new tab"
                            : "Open in new tab"
                        }
                        aria-label={
                          preferredKind === "pdf"
                            ? "Open PDF in new tab"
                            : "Open in new tab"
                        }
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

              <div className="flex-1 min-w-0 overflow-auto bg-muted/30 px-5 py-5 min-h-0">
                {isMarkdown ? (
                  markdownLoading && !markdownPreview ? (
                    <div className="flex items-center justify-center py-10 text-sm text-muted-foreground gap-2">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Loading preview…
                    </div>
                  ) : markdownPreview ? (
                    <div className="mx-auto w-full max-w-[820px] rounded-xl border border-border/60 bg-background p-6 shadow-sm">
                      <LazyMarkdownRenderer
                        content={normalizedMarkdown ?? markdownPreview}
                        containerClassName="markdown-compact"
                        className="prose text-base leading-normal text-foreground prose-ol:flex prose-ol:flex-col prose-ul:flex prose-ul:flex-col prose-li:flex prose-li:flex-col"
                      />
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">
                      Preview unavailable.
                    </div>
                  )
                ) : htmlAsset ? (
                  <div className="w-full space-y-4">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div className="flex items-center gap-2">
                        <Button
                          onClick={() => setHtmlView("preview")}
                          variant={
                            htmlView === "preview" ? "default" : "outline"
                          }
                          size="sm"
                        >
                          Preview
                        </Button>
                        <Button
                          onClick={() => setHtmlView("source")}
                          variant={
                            htmlView === "source" ? "default" : "outline"
                          }
                          size="sm"
                        >
                          Source
                        </Button>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        {htmlReady && (
                          <Badge
                            variant={
                              htmlStatus === "error"
                                ? "destructive"
                                : htmlStatus === "warning"
                                  ? "warning"
                                  : "success"
                            }
                          >
                            {htmlStatus === "error"
                              ? `Errors (${htmlErrors.length})`
                              : htmlStatus === "warning"
                                ? `Warnings (${htmlWarnings.length})`
                                : "Valid HTML"}
                          </Badge>
                        )}
                        {isHtmlDirty && <Badge variant="warning">Edited</Badge>}
                        <Button
                          onClick={() => setHtmlDraft(null)}
                          variant="outline"
                          size="sm"
                          disabled={!isHtmlDirty}
                        >
                          Reset
                        </Button>
                      </div>
                    </div>

                    {htmlView === "preview" ? (
                      htmlPreviewUri ? (
                        <div className="w-full">
                          <div className="mx-auto w-full max-w-[1100px]">
                            <iframe
                              src={htmlPreviewUri}
                              className="h-[80vh] w-full rounded-xl border border-border/60 bg-white"
                              title="Preview"
                            />
                          </div>
                        </div>
                      ) : (
                        <div className="text-sm text-muted-foreground">
                          Preview unavailable.
                        </div>
                      )
                    ) : htmlLoading && !htmlSource ? (
                      <div className="flex items-center justify-center py-10 text-sm text-muted-foreground gap-2">
                        <Loader2 className="h-4 w-4 animate-spin" />
                        Loading HTML...
                      </div>
                    ) : htmlError ? (
                      <div className="rounded-xl border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive">
                        {htmlError}
                      </div>
                    ) : (
                      <div className="space-y-3">
                        <Textarea
                          value={htmlContent}
                          onChange={(event) => setHtmlDraft(event.target.value)}
                          spellCheck={false}
                          className={cn(
                            "font-mono text-xs leading-relaxed resize-y",
                            HTML_EDITOR_MIN_HEIGHT,
                          )}
                        />
                        <div className="rounded-xl border border-border/60 bg-background p-3">
                          <div className="flex items-center gap-2 text-xs font-semibold text-foreground">
                            <span>Validation</span>
                            {htmlReady && (
                              <Badge
                                variant={
                                  htmlStatus === "error"
                                    ? "destructive"
                                    : htmlStatus === "warning"
                                      ? "warning"
                                      : "success"
                                }
                                className="text-[10px]"
                              >
                                {htmlStatus === "error"
                                  ? `${htmlErrors.length} errors`
                                  : htmlStatus === "warning"
                                    ? `${htmlWarnings.length} warnings`
                                    : "OK"}
                              </Badge>
                            )}
                          </div>
                          {!htmlReady ? (
                            <p className="mt-2 text-xs text-muted-foreground">
                              Load HTML to validate.
                            </p>
                          ) : htmlIssues.length === 0 ? (
                            <p className="mt-2 text-xs text-emerald-700">
                              No validation issues found.
                            </p>
                          ) : (
                            <ul className="mt-2 space-y-1 text-xs">
                              {htmlIssues.map((issue, index) => (
                                <li
                                  key={`html-issue-${index}`}
                                  className={
                                    issue.level === "error"
                                      ? "text-destructive"
                                      : "text-amber-700"
                                  }
                                >
                                  {issue.level.toUpperCase()}: {issue.message}
                                </li>
                              ))}
                            </ul>
                          )}
                        </div>
                      </div>
                    )}
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
