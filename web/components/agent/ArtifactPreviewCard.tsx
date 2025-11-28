"use client";

import { useEffect, useState } from "react";

import { AttachmentPayload, AttachmentPreviewAssetPayload } from "@/lib/types";
import { buildAttachmentUri } from "@/lib/attachments";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ImagePreview } from "@/components/ui/image-preview";
import { MarkdownRenderer } from "@/components/ui/markdown";
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

export function ArtifactPreviewCard({
  attachment,
  className,
}: ArtifactPreviewCardProps) {
  const [markdownPreview, setMarkdownPreview] = useState<string | null>(null);
  const [markdownLoading, setMarkdownLoading] = useState(false);
  const [markdownError, setMarkdownError] = useState<string | null>(null);
  const [isExpanded, setIsExpanded] = useState(false);

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
    if (!isMarkdown || !downloadUri) {
      setMarkdownPreview(null);
      setMarkdownError(null);
      return;
    }
    let cancelled = false;
    setMarkdownLoading(true);
    setMarkdownError(null);

    fetch(downloadUri)
      .then((resp) => {
        if (!resp.ok) {
          throw new Error(`HTTP ${resp.status}`);
        }
        return resp.text();
      })
      .then((text) => {
        if (!cancelled) {
          setMarkdownPreview(text);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setMarkdownError(err instanceof Error ? err.message : "Failed to load preview");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setMarkdownLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [downloadUri, isMarkdown]);

  return (
    <div className={cn("rounded-2xl border border-border bg-card p-3 space-y-3 shadow-sm", className)}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-foreground">{displayName}</p>
          {formatLabel && (
            <p className="mt-0.5 text-[10px] font-semibold uppercase tracking-[0.3em] text-muted-foreground">
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
            <a
              href={downloadUri}
              download={attachment.name || "download"}
              className="inline-flex items-center justify-center rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-8 px-3 text-sm"
            >
              <Download className="mr-1 h-3.5 w-3.5" />
              Download
            </a>
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
          <MarkdownRenderer
            content={markdownPreview}
            containerClassName="markdown-body text-sm"
            className="prose-sm max-w-none"
          />
          {isExpanded && (
            <div className="mt-2 text-[11px] uppercase tracking-[0.2em] text-muted-foreground">
              End of preview
            </div>
          )}
        </div>
      ) : isMarkdown && markdownLoading ? (
        <div className="rounded-xl border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
          <p>Loading preview…</p>
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

    </div>
  );
}
