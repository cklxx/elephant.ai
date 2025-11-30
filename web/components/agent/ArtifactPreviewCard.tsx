"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { AttachmentPayload, AttachmentPreviewAssetPayload } from "@/lib/types";
import { buildAttachmentUri } from "@/lib/attachments";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ImagePreview } from "@/components/ui/image-preview";
import { LazyMarkdownRenderer } from "@/components/agent/LazyMarkdownRenderer";
import { Download, Maximize2, Minimize2 } from "lucide-react";

interface ArtifactPreviewCardProps {
  attachment: AttachmentPayload;
  className?: string;
}

function normalizeAssets(assets?: AttachmentPreviewAssetPayload[] | null) {
  if (!assets || assets.length === 0) {
    return [];
  }
  return assets.filter((asset): asset is AttachmentPreviewAssetPayload & { cdn_url: string } => {
    return Boolean(asset && asset.cdn_url);
  });
}

function decodeBase64ToText(value: string): string | null {
  try {
    const binary = atob(value);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    return new TextDecoder().decode(bytes);
  } catch {
    return null;
  }
}

function decodeDataUriToText(uri: string): string | null {
  const trimmed = uri.trim();
  if (!trimmed.startsWith("data:")) {
    return null;
  }

  const commaIndex = trimmed.indexOf(",");
  if (commaIndex === -1) {
    return null;
  }

  const meta = trimmed.slice(5, commaIndex);
  const payload = trimmed.slice(commaIndex + 1);
  const isBase64 = meta.includes(";base64");

  if (isBase64) {
    return decodeBase64ToText(payload);
  }

  try {
    return decodeURIComponent(payload);
  } catch {
    return payload || null;
  }
}

function extractInlineMarkdown(attachment: AttachmentPayload): string | null {
  const inlineData = attachment.data?.trim();
  if (inlineData) {
    if (inlineData.startsWith("data:")) {
      const decoded = decodeDataUriToText(inlineData);
      if (decoded) {
        return decoded;
      }
    }
    const decoded = decodeBase64ToText(inlineData);
    if (decoded) {
      return decoded;
    }
  }

  const inlineAsset = attachment.preview_assets?.find((asset) => {
    const url = asset.cdn_url?.trim();
    if (!url || !url.startsWith("data:")) {
      return false;
    }
    const hint = `${asset.mime_type || ""} ${asset.preview_type || ""}`.toLowerCase();
    return hint.includes("markdown") || hint.includes("text");
  });

  if (inlineAsset?.cdn_url) {
    const decoded = decodeDataUriToText(inlineAsset.cdn_url);
    if (decoded) {
      return decoded;
    }
  }

  return null;
}

export function ArtifactPreviewCard({
  attachment,
  className,
}: ArtifactPreviewCardProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [markdownPreview, setMarkdownPreview] = useState<string | null>(null);
  const [markdownLoading, setMarkdownLoading] = useState(false);
  const [markdownError, setMarkdownError] = useState<string | null>(null);
  const [isExpanded, setIsExpanded] = useState(false);
  const [shouldLoadMarkdown, setShouldLoadMarkdown] = useState(() => isExpanded);
  const inlineMarkdown = useMemo(() => extractInlineMarkdown(attachment), [attachment]);

  const previewAssets = normalizeAssets(attachment.preview_assets);
  const imageAssets = previewAssets.filter((asset) => {
    const mime = asset.mime_type?.toLowerCase() ?? "";
    const previewType = asset.preview_type?.toLowerCase() ?? "";
    return mime.startsWith("image/") || previewType.includes("image");
  });
  const htmlAsset = previewAssets.find((asset) => {
    const mime = asset.mime_type?.toLowerCase() ?? "";
    const previewType = asset.preview_type?.toLowerCase() ?? "";
    return mime.includes("html") || previewType.includes("html") || previewType.includes("iframe");
  });

  const downloadUri = buildAttachmentUri(attachment);
  const displayName = attachment.description || attachment.name || "Artifact";
  const formatLabel = attachment.format
    ? attachment.format.toUpperCase()
    : attachment.media_type;
  const isMarkdown =
    (attachment.format || "").toLowerCase() === "markdown" ||
    (attachment.media_type || "").toLowerCase().includes("markdown");
  const canInlinePreview = Boolean(htmlAsset) || isMarkdown;
  const shouldShowExpand = canInlinePreview;
  const expandLabel = isExpanded ? "Collapse" : "Expand";
  const markdownContainerClasses = cn(
    "relative rounded-xl border border-border/60 bg-background p-4 overflow-auto",
    isExpanded ? "max-h-[70vh]" : "max-h-72",
  );

  useEffect(() => {
    // Reset when attachment changes
    setMarkdownPreview(null);
    setMarkdownError(null);
    setMarkdownLoading(false);
    setShouldLoadMarkdown(false);
  }, [downloadUri, isMarkdown]);

  useEffect(() => {
    if (isMarkdown && isExpanded) {
      setShouldLoadMarkdown(true);
    }
  }, [isMarkdown, isExpanded]);

  useEffect(() => {
    if (!isMarkdown || shouldLoadMarkdown || isExpanded) {
      return;
    }

    const node = containerRef.current;
    if (!node || typeof IntersectionObserver === "undefined") {
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry && entry.isIntersecting) {
          setShouldLoadMarkdown(true);
        }
      },
      { rootMargin: "200px 0px" },
    );

    observer.observe(node);
    return () => observer.disconnect();
  }, [isMarkdown, isExpanded, shouldLoadMarkdown]);

  useEffect(() => {
    if (!isMarkdown || !shouldLoadMarkdown) {
      return;
    }

    let cancelled = false;
    setMarkdownLoading(true);
    setMarkdownError(null);

    const applyInlineMarkdown = () => {
      if (!inlineMarkdown) {
        return false;
      }
      if (!cancelled) {
        setMarkdownPreview(inlineMarkdown);
        setMarkdownError(null);
      }
      return true;
    };

    const loadMarkdown = async () => {
      if (downloadUri) {
        try {
          const resp = await fetch(downloadUri);
          if (!resp.ok) {
            throw new Error(`HTTP ${resp.status}`);
          }
          const text = await resp.text();
          if (!cancelled) {
            setMarkdownPreview(text);
          }
        } catch (err) {
          if (!cancelled && applyInlineMarkdown()) {
            return;
          }
          if (!cancelled) {
            setMarkdownError(err instanceof Error ? err.message : "Failed to load preview");
          }
        } finally {
          if (!cancelled) {
            setMarkdownLoading(false);
          }
        }
        return;
      }

      if (applyInlineMarkdown()) {
        if (!cancelled) {
          setMarkdownLoading(false);
        }
        return;
      }

      if (!cancelled) {
        setMarkdownLoading(false);
        setMarkdownError("Preview unavailable");
      }
    };

    void loadMarkdown();

    return () => {
      cancelled = true;
    };
  }, [downloadUri, inlineMarkdown, isMarkdown, shouldLoadMarkdown]);

  return (
    <Card ref={containerRef} className={cn("rounded-2xl border border-border bg-card", className)}>
      <CardContent className="space-y-3 p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-sm font-semibold text-foreground">{displayName}</p>
            {formatLabel && (
              <p className="mt-0.5 text-[10px] font-semibold text-muted-foreground">
                {formatLabel}
              </p>
            )}
          </div>
          <div className="flex items-center gap-2">
            {shouldShowExpand && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-8 px-3"
                onClick={() => setIsExpanded((prev) => !prev)}
              >
                {isExpanded ? (
                  <Minimize2 className="mr-1.5 h-4 w-4" />
                ) : (
                  <Maximize2 className="mr-1.5 h-4 w-4" />
                )}
                {expandLabel}
              </Button>
            )}
            {downloadUri && (
              <Button asChild variant="outline" size="sm" className="h-8 px-3">
                <a href={downloadUri} target="_blank" rel="noreferrer">
                  View
                </a>
              </Button>
            )}
            {downloadUri && (
              <Button asChild size="sm" className="h-8 px-3">
                <a href={downloadUri} download={attachment.name || "download"}>
                  <Download className="mr-1 h-3.5 w-3.5" />
                  Download
                </a>
              </Button>
            )}
          </div>
        </div>

      {imageAssets.length > 0 ? (
        <>
          <ImagePreview
            src={imageAssets[0].cdn_url}
            alt={`${displayName} preview`}
            minHeight="12rem"
            maxHeight="20rem"
            className="rounded-2xl border border-border/40"
          />
          {imageAssets.length > 1 && (
            <div className="grid grid-cols-3 gap-2">
              {imageAssets.slice(1).map((asset, index) => (
                <ImagePreview
                  key={`artifact-thumb-${asset.asset_id || index}`}
                  src={asset.cdn_url}
                  alt={`${displayName} preview ${index + 2}`}
                  minHeight="4.5rem"
                  maxHeight="6rem"
                  className="rounded-xl border border-border/40 bg-muted/30"
                  imageClassName="object-cover"
                />
              ))}
            </div>
          )}
        </>
      ) : htmlAsset ? (
        isExpanded ? (
          <div className="rounded-xl border border-border/60 overflow-hidden bg-background">
            <iframe
              src={htmlAsset.cdn_url}
              title={`${displayName} preview`}
              sandbox="allow-same-origin allow-scripts"
              className="h-[70vh] w-full"
            />
          </div>
        ) : (
          <div className="rounded-xl border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
            <p>Expand to load the full preview.</p>
          </div>
        )
      ) : isMarkdown && markdownPreview ? (
        <div className={markdownContainerClasses}>
          <LazyMarkdownRenderer
            content={markdownPreview}
            containerClassName="markdown-body text-sm"
            className="prose-sm max-w-none"
          />
          {isExpanded && (
            <div className="mt-2 text-[11px] font-medium text-muted-foreground">
              End of preview
            </div>
          )}
        </div>
      ) : isMarkdown && markdownLoading ? (
        <div className="rounded-xl border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
          <p>Loading preview…</p>
        </div>
      ) : isMarkdown && !shouldLoadMarkdown ? (
        <div className="rounded-xl border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
          <p>Scroll into view or expand to load the preview.</p>
        </div>
      ) : isMarkdown && markdownError ? (
        <div className="rounded-xl border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
          <p>Preview unavailable: {markdownError}</p>
        </div>
      ) : (
        <div className="rounded-xl border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
          <p>No preview available. Use Download to save this artifact.</p>
        </div>
      )}

      </CardContent>
    </Card>
  );
}
