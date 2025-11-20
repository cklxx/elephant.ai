"use client";

import { AttachmentPayload, AttachmentPreviewAssetPayload } from "@/lib/types";
import { buildAttachmentUri } from "@/lib/attachments";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ImagePreview } from "@/components/ui/image-preview";
import { Download, ExternalLink } from "lucide-react";

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
  const viewUri = htmlAsset?.cdn_url || imageAssets[0]?.cdn_url || downloadUri;
  const displayName = attachment.description || attachment.name || "Artifact";
  const formatLabel = attachment.format
    ? attachment.format.toUpperCase()
    : attachment.media_type;

  return (
    <div className={cn("rounded-2xl border border-border/70 bg-card/70 p-3 space-y-3 shadow-sm", className)}>
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
          {viewUri && (
            <a
              href={viewUri}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center justify-center rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 border border-primary/40 text-primary hover:bg-primary/10 h-8 px-3 text-sm"
            >
              <ExternalLink className="mr-1 h-3.5 w-3.5" />
              View
            </a>
          )}
          {downloadUri && downloadUri !== viewUri && (
            <a
              href={downloadUri}
              target="_blank"
              rel="noreferrer"
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
        <div className="rounded-xl border border-border/60 overflow-hidden bg-background">
          <iframe
            src={htmlAsset.cdn_url}
            title={`${displayName} preview`}
            sandbox="allow-same-origin allow-scripts"
            className="h-64 w-full"
          />
        </div>
      ) : viewUri ? (
        <div className="rounded-xl border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
          <p>Preview unavailable. Use the actions above to open the artifact.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-dashed border-border/70 p-4 text-xs text-muted-foreground">
          <p>No preview available for this artifact.</p>
        </div>
      )}

      {previewAssets.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {previewAssets.map((asset, index) => (
            <a
              key={`artifact-asset-${asset.asset_id || index}`}
              href={asset.cdn_url}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 rounded-full border border-border/70 px-2 py-1 text-[10px] uppercase tracking-[0.2em] text-muted-foreground transition hover:bg-muted/40"
            >
              <ExternalLink className="h-3 w-3" />
              {asset.label || `Preview ${index + 1}`}
            </a>
          ))}
        </div>
      )}
    </div>
  );
}
