'use client';

import { useState, useRef, useEffect } from 'react';
import Image from 'next/image';
import { AttachmentPayload } from '@/lib/types';
import { cn } from '@/lib/utils';
import { buildAttachmentUri } from '@/lib/attachments';
import { LazyMarkdownRenderer } from "@/components/agent/LazyMarkdownRenderer";
import { Download, ExternalLink, FileText, FileCode, ChevronDown, Eye, Loader2 } from "lucide-react";

interface ArtifactPreviewCardProps {
  attachment: AttachmentPayload;
  className?: string;
}

export function ArtifactPreviewCard({
  attachment,
  className,
}: ArtifactPreviewCardProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [markdownPreview, setMarkdownPreview] = useState<string | null>(null);
  const [markdownLoading, setMarkdownLoading] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  // Auto-expand if it's small or we want immediate preview
  const [shouldLoadMarkdown, setShouldLoadMarkdown] = useState(false);

  const downloadUri = buildAttachmentUri(attachment);
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

  // Effect to load markdown
  useEffect(() => {
    if (!isMarkdown || !shouldLoadMarkdown) return;

    // Quick inline check (if data URI)
    if (attachment.data?.startsWith("data:")) {
      // Decode logic handled elsewhere or assume text for now
    }

    let cancelled = false;
    setMarkdownLoading(true);

    async function fetchMd() {
      if (!downloadUri) return;
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
  }, [downloadUri, isMarkdown, shouldLoadMarkdown, attachment.data]);

  return (
    <div className={cn(
      "group relative overflow-hidden rounded-xl border border-border/40 bg-card transition-colors max-w-md my-2",
      className
    )}>
      {/* Header / Main Body - Manus Style File Card */}
      <div className="flex items-center gap-3 p-3">
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
          <h4 className="text-sm font-medium text-foreground truncate">{displayName}</h4>
          <div className="flex items-center gap-2 text-xs text-muted-foreground/80">
            <span className="uppercase">{formatLabel}</span>
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
          {canInlinePreview && (
            <button
              onClick={() => {
                setShouldLoadMarkdown(true); // Ensure load trigger
                setIsExpanded(!isExpanded);
              }}
              className="p-2 rounded-md hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
              title="Preview"
              aria-label="Preview"
            >
              {isExpanded ? <ChevronDown className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          )}

          {downloadUri && (
            <>
              <a
                href={downloadUri}
                className="p-2 rounded-md hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
                title="View"
                aria-label="View"
                target="_blank"
                rel="noopener noreferrer"
              >
                <ExternalLink className="w-4 h-4" />
              </a>
              <a
                href={downloadUri}
                download={attachment.name}
                className="p-2 rounded-md hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
                title="Download"
                aria-label="Download"
              >
                <Download className="w-4 h-4" />
              </a>
            </>
          )}
        </div>
      </div>

      {/* Preview Area */}
      {isExpanded && (
        <div className="border-t border-border/40 bg-muted/10">
          {markdownLoading ? (
            <div className="flex items-center justify-center py-8 text-muted-foreground text-sm gap-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              Loading preview...
            </div>
          ) : markdownPreview ? (
            <div className="relative max-h-[400px] overflow-y-auto p-4 text-sm prose prose-sm dark:prose-invert max-w-none">
              <LazyMarkdownRenderer content={markdownPreview} />
            </div>
          ) : htmlAsset ? (
            <iframe
              src={htmlAsset.cdn_url}
              className="w-full h-[400px] border-none bg-white"
              title="Preview"
            />
          ) : (
            <div className="p-4 text-center text-sm text-muted-foreground">
              No preview available
            </div>
          )}
        </div>
      )}
    </div>
  );
}
