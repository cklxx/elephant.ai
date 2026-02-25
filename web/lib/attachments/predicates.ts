import type { AttachmentPayload } from "@/lib/types";
import type { AttachmentSegmentType } from "./types";

const VIDEO_EXTENSIONS = new Set([
  "mp4",
  "mov",
  "webm",
  "mkv",
  "avi",
  "m4v",
  "mpeg",
  "mpg",
]);
const HTML_EXTENSIONS = new Set(["html", "htm"]);
const DOCUMENT_FORMATS = new Set([
  "ppt",
  "pptx",
  "pdf",
  "markdown",
  "md",
  "mkd",
  "mdown",
  "doc",
  "docx",
]);

function extractAttachmentExtension(value: string | undefined | null): string | null {
  if (!value) return null;
  const trimmed = value.trim();
  if (!trimmed) return null;
  const withoutQuery = trimmed.split(/[?#]/)[0] ?? trimmed;
  const filename = withoutQuery.split("/").pop() ?? withoutQuery;
  const match = filename.match(/\.([^.]+)$/);
  if (!match?.[1]) {
    return null;
  }
  return match[1].toLowerCase();
}

export function getAttachmentSegmentType(
  attachment: AttachmentPayload,
): AttachmentSegmentType {
  const mediaType = attachment.media_type?.toLowerCase() ?? "";
  const mediaSubtype =
    mediaType.includes("/") && mediaType.split("/").length > 1
      ? mediaType.split("/")[1]?.split(";")[0]
      : "";
  const inferredExtension =
    extractAttachmentExtension(attachment.name) ??
    extractAttachmentExtension(attachment.uri);
  const inferredHtml = inferredExtension
    ? HTML_EXTENSIONS.has(inferredExtension)
    : false;
  const inferredVideo = inferredExtension
    ? VIDEO_EXTENSIONS.has(inferredExtension)
    : false;
  const inferredDocument = inferredExtension
    ? DOCUMENT_FORMATS.has(inferredExtension)
    : false;

  if (mediaType.startsWith("video/") || inferredVideo) {
    return "video";
  }

  const format = attachment.format?.toLowerCase();
  const normalizedFormat = format || mediaSubtype;
  const previewProfile = attachment.preview_profile?.toLowerCase() ?? "";
  const kind = attachment.kind?.toLowerCase();
  const previewAssets = attachment.preview_assets ?? [];
  const hasPreviewAssets = previewAssets.length > 0;
  const hasVideoAsset = previewAssets.some((asset) => {
    const mime = asset.mime_type?.toLowerCase() ?? "";
    const previewType = asset.preview_type?.toLowerCase() ?? "";
    return mime.startsWith("video/") || previewType.includes("video");
  });
  if (hasVideoAsset) {
    return "video";
  }
  const hasHtmlAsset = previewAssets.some((asset) => {
    const mime = asset.mime_type?.toLowerCase() ?? "";
    const previewType = asset.preview_type?.toLowerCase() ?? "";
    return (
      mime.includes("html") ||
      previewType.includes("html") ||
      previewType.includes("iframe")
    );
  });
  const htmlLike =
    mediaType === "text/html" ||
    normalizedFormat === "html" ||
    previewProfile.includes("html") ||
    hasHtmlAsset ||
    inferredHtml;
  if (htmlLike) {
    return "embed";
  }

  const documentProfile = previewProfile.startsWith("document.");
  const markdownFormat =
    normalizedFormat === "markdown" ||
    normalizedFormat === "md" ||
    normalizedFormat === "mkd" ||
    normalizedFormat === "mdown" ||
    normalizedFormat === "x-markdown";
  const documentFormat =
    inferredDocument ||
    (normalizedFormat && DOCUMENT_FORMATS.has(normalizedFormat)) ||
    normalizedFormat === "pdf" ||
    markdownFormat;
  const isArtifact = kind === "artifact";
  if (
    isArtifact ||
    documentProfile ||
    documentFormat ||
    hasPreviewAssets ||
    (mediaType.startsWith("text/") && !htmlLike)
  ) {
    return "document";
  }

  return "image";
}

export function isA2UIAttachment(attachment: AttachmentPayload): boolean {
  const mediaType = attachment.media_type?.toLowerCase() ?? "";
  const format = attachment.format?.toLowerCase() ?? "";
  const previewProfile = attachment.preview_profile?.toLowerCase() ?? "";
  return (
    mediaType.includes("a2ui") || format === "a2ui" || previewProfile.includes("a2ui")
  );
}
