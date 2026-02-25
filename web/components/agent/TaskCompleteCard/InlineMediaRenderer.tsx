import type { ReactNode } from "react";

import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "@/components/agent/ArtifactPreviewCard";
import { buildAttachmentUri, type ContentSegment } from "@/lib/attachments";
import type { AttachmentPayload } from "@/lib/types";

export interface InlineAttachmentEntry {
  key: string;
  type: string;
  description?: string;
  mime?: string;
  attachment: AttachmentPayload;
}

interface InlineMarkdownImageProps {
  src?: string;
  alt?: string;
}

export function InlineMarkdownImage({ src, alt }: InlineMarkdownImageProps) {
  if (!src) {
    return null;
  }
  return (
    <ImagePreview
      src={src}
      alt={alt}
      className="my-2 mr-3 inline-block w-[220px] max-w-full align-middle"
      minHeight="8rem"
      maxHeight="14rem"
      sizes="220px"
      imageClassName="object-contain"
      loading="eager"
    />
  );
}

export function renderInlineAttachment(segment: ContentSegment, index: number) {
  if (!segment.attachment) {
    return null;
  }
  const uri = buildAttachmentUri(segment.attachment);
  if (!uri) {
    return null;
  }
  const altText =
    segment.attachment.description ||
    segment.attachment.name ||
    `attachment-${index + 1}`;
  if (segment.type === "video") {
    return (
      <VideoPreview
        key={`task-complete-inline-video-${segment.placeholder ?? index}`}
        src={uri}
        mimeType={segment.attachment.media_type || "video/mp4"}
        description={segment.attachment.description}
        className="w-full"
        maxHeight="20rem"
      />
    );
  }
  if (segment.type === "document" || segment.type === "embed") {
    return (
      <div key={`task-complete-inline-doc-${segment.placeholder ?? index}`} className="my-2">
        <ArtifactPreviewCard attachment={segment.attachment} compact />
      </div>
    );
  }
  return (
    <InlineMarkdownImage
      key={`task-complete-inline-image-${segment.placeholder ?? index}`}
      src={uri}
      alt={altText}
    />
  );
}

export function createInlineMarkdownLink(
  inlineAttachmentMap: Map<string, InlineAttachmentEntry>,
) {
  return function InlineMarkdownLink({
    href,
    children,
    ...props
  }: {
    href?: string;
    children?: ReactNode;
  }) {
    const resolvedHref = href?.trim() ?? "";
    if (!resolvedHref) {
      return <span {...props}>{children}</span>;
    }
    const matchedAttachment = inlineAttachmentMap.get(resolvedHref);
    if (
      matchedAttachment &&
      (matchedAttachment.type === "document" ||
        matchedAttachment.type === "embed")
    ) {
      return (
        <div className="my-2">
          <ArtifactPreviewCard attachment={matchedAttachment.attachment} compact />
        </div>
      );
    }
    if (matchedAttachment?.type === "image") {
      const altText =
        matchedAttachment.description ||
        (typeof children === "string" ? children : undefined) ||
        matchedAttachment.key;
      return <InlineMarkdownImage src={resolvedHref} alt={altText} />;
    }
    if (matchedAttachment?.type === "video") {
      return (
        <VideoPreview
          src={resolvedHref}
          mimeType={matchedAttachment.mime || "video/mp4"}
          description={
            matchedAttachment.description ||
            (typeof children === "string" ? children : undefined) ||
            matchedAttachment.key
          }
          className="my-2 w-full"
          maxHeight="20rem"
        />
      );
    }
    return (
      <a className="break-words whitespace-normal" href={resolvedHref} {...props}>
        {children}
      </a>
    );
  };
}

export function createInlineImageRenderer(
  inlineAttachmentMap: Map<string, InlineAttachmentEntry>,
  inlineImageMap: Map<string, string>,
) {
  return function InlineMarkdownImageRenderer({
    src,
    alt,
  }: {
    src?: string;
    alt?: string;
  }) {
    const recoveredSrc =
      (src && src.trim()) ||
      inlineImageMap.get((alt || "").trim()) ||
      undefined;
    if (!recoveredSrc) {
      return null;
    }
    const matchedAttachment = inlineAttachmentMap.get(recoveredSrc);
    if (matchedAttachment?.type === "video") {
      return (
        <VideoPreview
          key={`task-complete-inline-video-${matchedAttachment.key}`}
          src={recoveredSrc}
          mimeType={matchedAttachment.mime || "video/mp4"}
          description={
            matchedAttachment.description || alt || matchedAttachment.key
          }
          className="w-full"
          maxHeight="20rem"
        />
      );
    }

    if (
      matchedAttachment &&
      (matchedAttachment.type === "document" ||
        matchedAttachment.type === "embed")
    ) {
      return (
        <div className="my-2">
          <ArtifactPreviewCard attachment={matchedAttachment.attachment} compact />
        </div>
      );
    }

    return <InlineMarkdownImage src={recoveredSrc} alt={alt} />;
  };
}
