import { useMemo } from "react";

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
  const segments = useMemo(
    () => parseContentSegments(markdownAnswer, attachments),
    [attachments, markdownAnswer],
  );

  const inlineRenderBlocks = useMemo(() => {
    const blocks: InlineRenderBlock[] = [];
    let buffer = "";

    segments.forEach((segment) => {
      if (segment.type === "text") {
        if (segment.text) {
          buffer += segment.text;
        }
        return;
      }

      if (segment.implicit) {
        return;
      }

      const placeholder = segment.placeholder?.trim() ?? "";
      if (!placeholder.startsWith("[")) {
        if (placeholder) {
          buffer += placeholder;
        }
        return;
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
    });

    if (buffer.length > 0) {
      blocks.push({ kind: "markdown", content: buffer });
    }

    if (blocks.length === 0) {
      blocks.push({ kind: "markdown", content: markdownAnswer });
    }

    return blocks;
  }, [markdownAnswer, segments]);

  const { unreferencedMediaSegments, artifactSegments } = useMemo(() => {
    const unreferencedMedia: ContentSegment[] = [];
    const artifacts: ContentSegment[] = [];

    for (const segment of segments) {
      if (!segment.attachment) continue;

      if (!segment.implicit) {
        continue;
      }

      if (segment.type === "image" || segment.type === "video") {
        unreferencedMedia.push(segment);
      } else if (segment.type === "document" || segment.type === "embed") {
        artifacts.push(segment);
      }
    }

    return {
      unreferencedMediaSegments: unreferencedMedia,
      artifactSegments: artifacts,
    };
  }, [segments]);

  const shouldSoftenSummary = useMemo(() => {
    const text = markdownAnswer.trim();
    if (!text) {
      return true;
    }
    const headingMatches = text.match(/^#{1,6}\s+/gm) ?? [];
    const listMatches = text.match(/^\s*(?:[-*+]|\d+\.)\s+/gm) ?? [];
    const headingCount = headingMatches.length;
    const listCount = listMatches.length;
    const isDocumentLike =
      headingCount >= 3 || listCount >= 6 || text.length >= 800;
    return !isDocumentLike;
  }, [markdownAnswer]);

  return {
    inlineRenderBlocks,
    unreferencedMediaSegments,
    artifactSegments,
    hasMultipleArtifacts: artifactSegments.length > 1,
    shouldSoftenSummary,
  };
}
