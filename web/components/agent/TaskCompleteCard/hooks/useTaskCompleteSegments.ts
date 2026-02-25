import { parseContentSegments, type ContentSegment } from "@/lib/attachments";
import type { AttachmentPayload } from "@/lib/types";

export type InlineRenderBlock =
  | { kind: "markdown"; content: string }
  | { kind: "attachment"; segment: ContentSegment };

interface UseTaskCompleteSegmentsOptions {
  markdownAnswer: string;
  attachments?: Record<string, AttachmentPayload>;
}

export function useTaskCompleteSegments({
  markdownAnswer,
  attachments,
}: UseTaskCompleteSegmentsOptions) {
  // Parse segments from markdown and attachments
  const segments = parseContentSegments(markdownAnswer, attachments);

  // Build inline render blocks from segments
  const blocks: InlineRenderBlock[] = [];
  let buffer = "";

  for (const segment of segments) {
    if (segment.type === "text") {
      if (segment.text) {
        buffer += segment.text;
      }
      continue;
    }

    if (segment.implicit) {
      continue;
    }

    const placeholder = segment.placeholder?.trim() ?? "";
    if (!placeholder.startsWith("[")) {
      if (placeholder) {
        buffer += placeholder;
      }
      continue;
    }

    if (buffer.trim().length > 0) {
      blocks.push({ kind: "markdown", content: buffer });
      buffer = "";
    } else if (buffer.length > 0) {
      blocks.push({ kind: "markdown", content: buffer });
      buffer = "";
    }

    if (segment.attachment) {
      blocks.push({ kind: "attachment", segment });
    }
  }

  if (buffer.length > 0) {
    blocks.push({ kind: "markdown", content: buffer });
  }

  if (blocks.length === 0) {
    blocks.push({ kind: "markdown", content: markdownAnswer });
  }

  // Collect unreferenced media and artifacts
  const unreferencedMediaSegments: ContentSegment[] = [];
  const artifactSegments: ContentSegment[] = [];

  for (const segment of segments) {
    if (!segment.attachment) continue;
    if (!segment.implicit) continue;

    if (segment.type === "image" || segment.type === "video") {
      unreferencedMediaSegments.push(segment);
    } else if (segment.type === "document" || segment.type === "embed") {
      artifactSegments.push(segment);
    }
  }

  // Determine if summary should be softened
  const text = markdownAnswer.trim();
  const shouldSoftenSummary = !text
    ? true
    : (() => {
        const headingMatches = text.match(/^#{1,6}\s+/gm) ?? [];
        const listMatches = text.match(/^\s*(?:[-*+]|\d+\.)\s+/gm) ?? [];
        const headingCount = headingMatches.length;
        const listCount = listMatches.length;
        const isDocumentLike =
          headingCount >= 3 || listCount >= 6 || text.length >= 800;
        return !isDocumentLike;
      })();

  return {
    inlineRenderBlocks: blocks,
    unreferencedMediaSegments,
    artifactSegments,
    hasMultipleArtifacts: artifactSegments.length > 1,
    shouldSoftenSummary,
  };
}
