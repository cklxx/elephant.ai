"use client";

import { useMemo } from "react";
import type { ReactNode } from "react";

import { WorkflowResultFinalEvent, AttachmentPayload } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";
import {
  parseContentSegments,
  buildAttachmentUri,
  replacePlaceholdersWithMarkdown,
  getAttachmentSegmentType,
  ContentSegment,
  isA2UIAttachment,
} from "@/lib/attachments";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "./ArtifactPreviewCard";
import { Card, CardContent } from "@/components/ui/card";
import { AgentMarkdown } from "./AgentMarkdown";
import { LoadingDots } from "@/components/ui/loading-states";
import { A2UIAttachmentPreview } from "@/components/agent/A2UIAttachmentPreview";
import { cn } from "@/lib/utils";

interface StopReasonCopy {
  title: string;
  body?: string;
}

interface TaskCompleteCardProps {
  event: WorkflowResultFinalEvent;
}

const INLINE_MEDIA_REGEX = /(data:image\/[^\s)]+|\/api\/data\/[A-Za-z0-9]+)/g;

function convertInlineMediaToMarkdown(text: string): string {
  if (!text) return text;
  let result = text;
  const matches = Array.from(text.matchAll(INLINE_MEDIA_REGEX));
  // Replace from the end to keep indexes stable
  for (let i = matches.length - 1; i >= 0; i -= 1) {
    const m = matches[i];
    if (!m || m.index === undefined) continue;
    const start = m.index;
    const end = start + m[0].length;
    const before = text.slice(Math.max(0, start - 3), start);
    if (before.includes("![")) {
      continue;
    }
    result = `${result.slice(0, start)}![inline media](${m[0]})${result.slice(end)}`;
  }
  return result;
}

export function TaskCompleteCard({ event }: TaskCompleteCardProps) {
  const t = useTranslation();
  const answer = event.final_answer ?? "";
  const markdownAnswer = convertInlineMediaToMarkdown(answer);
  const attachments = event.attachments ?? undefined;
  const { a2uiAttachments, standardAttachments } = useMemo(() => {
    if (!attachments) {
      return {
        a2uiAttachments: {} as Record<string, AttachmentPayload>,
        standardAttachments: undefined as
          | Record<string, AttachmentPayload>
          | undefined,
      };
    }
    const a2ui: Record<string, AttachmentPayload> = {};
    const standard: Record<string, AttachmentPayload> = {};
    Object.entries(attachments).forEach(([key, attachment]) => {
      if (isA2UIAttachment(attachment)) {
        a2ui[key] = attachment;
      } else {
        standard[key] = attachment;
      }
    });
    return {
      a2uiAttachments: a2ui,
      standardAttachments:
        Object.keys(standard).length > 0 ? standard : undefined,
    };
  }, [attachments]);
  const streamInProgress =
    event.stream_finished === false ||
    (event.is_streaming === true && event.stream_finished !== true);
  const streamFinished = event.stream_finished === true;
  const inlineAttachments = streamInProgress ? undefined : standardAttachments;
  const contentWithInlineMedia = replacePlaceholdersWithMarkdown(
    markdownAnswer,
    inlineAttachments,
  );
  const inlineImageMap = useMemo(() => {
    const map = new Map<string, string>();
    const imageRegex = /!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;
    let match: RegExpExecArray | null;
    while ((match = imageRegex.exec(contentWithInlineMedia)) !== null) {
      const alt = (match[1] || "").trim();
      const url = (match[2] || "").trim();
      if (!url) {
        continue;
      }
      const key = alt || url;
      if (!map.has(key)) {
        map.set(key, url);
      }
    }
    return map;
  }, [contentWithInlineMedia]);
  const { inlineAttachmentMap, attachmentNames, hasAttachments } =
    useMemo(() => {
      if (!inlineAttachments) {
        return {
          inlineAttachmentMap: new Map<
            string,
            {
              key: string;
              type: string;
              description?: string;
              mime?: string;
              attachment: NonNullable<
                WorkflowResultFinalEvent["attachments"]
              >[string];
            }
          >(),
          attachmentNames: [] as string[],
          hasAttachments: false,
        };
      }

      const inlineMap = new Map<
        string,
        {
          key: string;
          type: string;
          description?: string;
          mime?: string;
          attachment: NonNullable<
            WorkflowResultFinalEvent["attachments"]
          >[string];
        }
      >();
      const names: string[] = [];

      Object.entries(inlineAttachments).forEach(([key, attachment]) => {
        const uri = buildAttachmentUri(attachment);
        if (uri) {
          inlineMap.set(uri, {
            key,
            type: getAttachmentSegmentType(attachment),
            description: attachment.description,
            mime: attachment.media_type,
            attachment,
          });
        }
        const label = attachment.description || attachment.name || key;
        if (label) {
          names.push(label);
        }
      });

      return {
        inlineAttachmentMap: inlineMap,
        attachmentNames: names,
        hasAttachments: names.length > 0,
      };
    }, [inlineAttachments]);
  const { unreferencedMediaSegments, artifactSegments } = useMemo(() => {
    const segments = parseContentSegments(markdownAnswer, standardAttachments);
    const unreferencedMedia: ContentSegment[] = [];
    const artifacts: ContentSegment[] = [];

    for (const segment of segments) {
      if (!segment.attachment) continue;

      // If the segment is NOT implicit, it means it was found in the text (referenced).
      // We only want to show "unreferenced" items in the bottom grid.
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
  }, [markdownAnswer, standardAttachments]);
  const hasA2UIAttachments = Object.keys(a2uiAttachments).length > 0;
  const hasAnswerContent = contentWithInlineMedia.trim().length > 0;
  // Only render the markdown block when there is actual answer content (or the stream is still in progress).
  // Otherwise we may render an empty "final answer" area when the backend returns attachments-only results.
  const shouldRenderMarkdown = hasAnswerContent || streamInProgress;
  const hasUnrenderedAttachments =
    unreferencedMediaSegments.length > 0 || artifactSegments.length > 0;
  const shouldShowFallback =
    !shouldRenderMarkdown &&
    !hasUnrenderedAttachments &&
    !hasAttachments &&
    !hasA2UIAttachments;
  const shouldShowAttachmentNotice =
    !shouldRenderMarkdown &&
    !hasUnrenderedAttachments &&
    hasAttachments &&
    !hasA2UIAttachments;
  const hasMultipleArtifacts = artifactSegments.length > 1;
  const stopReasonCopy = getStopReasonCopy(event.stop_reason, t);
  const summaryComponents = useMemo(
    () => ({
      h1: (props: any) => (
        <div className="mt-2 font-medium text-foreground" {...props} />
      ),
      h2: (props: any) => (
        <div className="mt-2 font-medium text-foreground" {...props} />
      ),
      h3: (props: any) => (
        <div className="mt-2 font-medium text-foreground" {...props} />
      ),
      h4: (props: any) => (
        <div className="mt-2 font-medium text-foreground/90" {...props} />
      ),
      h5: (props: any) => (
        <div className="mt-2 font-medium text-foreground/80" {...props} />
      ),
      h6: (props: any) => (
        <div className="mt-2 font-medium text-foreground/70" {...props} />
      ),
      ul: (props: any) => <div className="my-2 space-y-1" {...props} />,
      ol: (props: any) => <div className="my-2 space-y-1" {...props} />,
      li: (props: any) => (
        <div className="whitespace-pre-wrap text-foreground" {...props} />
      ),
      blockquote: (props: any) => (
        <div
          className="my-2 border-l-2 border-border/40 pl-3 text-foreground/80"
          {...props}
        />
      ),
      hr: () => <div className="my-2 h-px w-full bg-border/40" />,
      strong: (props: any) => (
        <strong className="font-semibold text-foreground" {...props} />
      ),
    }),
    [],
  );
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

  const inlineRenderBlocks = useMemo(() => {
    const segments = parseContentSegments(markdownAnswer, standardAttachments);
    const blocks: Array<
      | { kind: "markdown"; content: string }
      | { kind: "attachment"; segment: ContentSegment }
    > = [];
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
  }, [markdownAnswer, standardAttachments]);

  const InlineMarkdownImage = ({
    src,
    alt,
  }: {
    src?: string;
    alt?: string;
  }) => {
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
  };

  const renderInlineAttachment = (segment: ContentSegment, index: number) => {
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
        <div
          key={`task-complete-inline-doc-${segment.placeholder ?? index}`}
          className="my-2"
        >
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
  };

  const InlineMarkdownLink = ({
    href,
    children,
    ...props
  }: {
    href?: string;
    children?: ReactNode;
  }) => {
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
      <a
        className="break-words whitespace-normal"
        href={resolvedHref}
        {...props}
      >
        {children}
      </a>
    );
  };

  return (
    <Card
      data-testid="task-complete-event"
      className="border-0 bg-transparent shadow-none"
    >
      {streamInProgress && !hasAnswerContent ? (
        <div
          className="rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-sm"
          data-testid="task-complete-streaming-placeholder"
        >
          <p className="font-medium text-foreground">
            {t("events.taskComplete.streaming")}
          </p>
          <p className="mt-1 inline-flex items-center gap-2 text-muted-foreground">
            <LoadingDots />
            <span>{t("events.taskComplete.streamingHint")}</span>
          </p>
        </div>
      ) : shouldRenderMarkdown ? (
        <>
          {streamInProgress ? (
            <AgentMarkdown
              content={contentWithInlineMedia}
              className="prose max-w-none text-base leading-snug text-foreground"
              attachments={inlineAttachments}
              isStreaming={streamInProgress}
              streamFinished={streamFinished}
              components={{
                ...(shouldSoftenSummary ? summaryComponents : {}),
                a: InlineMarkdownLink,
                img: ({ src, alt }: { src?: string; alt?: string }) => {
                  const recoveredSrc =
                    (src && src.trim()) ||
                    inlineImageMap.get((alt || "").trim()) ||
                    undefined;
                  if (!recoveredSrc) {
                    return null;
                  }
                  const matchedAttachment =
                    inlineAttachmentMap.get(recoveredSrc);
                  if (matchedAttachment?.type === "video") {
                    return (
                      <VideoPreview
                        key={`task-complete-inline-video-${matchedAttachment.key}`}
                        src={recoveredSrc}
                        mimeType={matchedAttachment.mime || "video/mp4"}
                        description={
                          matchedAttachment.description ||
                          alt ||
                          matchedAttachment.key
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
                        <ArtifactPreviewCard
                          attachment={matchedAttachment.attachment}
                          compact
                        />
                      </div>
                    );
                  }

                  return <InlineMarkdownImage src={recoveredSrc} alt={alt} />;
                },
              }}
            />
          ) : (
            <div className="space-y-2">
              {inlineRenderBlocks.map((block, index) => {
                if (block.kind === "attachment") {
                  return renderInlineAttachment(block.segment, index);
                }
                return (
                  <AgentMarkdown
                    key={`task-complete-inline-text-${index}`}
                    content={block.content}
                    className="prose max-w-none text-base leading-snug text-foreground"
                    isStreaming={false}
                    streamFinished
                    components={shouldSoftenSummary ? summaryComponents : {}}
                  />
                );
              })}
            </div>
          )}
        </>
      ) : shouldShowFallback ? (
        <div
          className="rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-sm"
          data-testid="task-complete-fallback"
        >
          <p className="font-medium text-foreground">{stopReasonCopy.title}</p>
          {stopReasonCopy.body && (
            <p className="mt-1 text-muted-foreground">{stopReasonCopy.body}</p>
          )}
        </div>
      ) : shouldShowAttachmentNotice ? (
        <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-sm">
          <p className="font-medium text-foreground">
            {t("events.taskComplete.empty")}
          </p>
          <p className="mt-1 text-muted-foreground">
            {t("events.taskComplete.attachmentsAvailable")}
          </p>
          {attachmentNames.length > 0 && (
            <ul className="mt-2 list-disc space-y-1 pl-4 text-xs text-muted-foreground">
              {attachmentNames.map((name) => (
                <li key={name}>{name}</li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {!streamInProgress && hasA2UIAttachments && (
        <div className="space-y-4">
          {Object.entries(a2uiAttachments).map(([key, attachment]) => (
            <A2UIAttachmentPreview
              key={`task-complete-a2ui-${key}`}
              attachment={attachment}
            />
          ))}
        </div>
      )}

      {!streamInProgress && unreferencedMediaSegments.length > 0 && (
        <div className="flex flex-wrap items-start gap-3">
          {unreferencedMediaSegments.map((segment, index) => {
            if (!segment.attachment) {
              return null;
            }
            const uri = buildAttachmentUri(segment.attachment);
            if (!uri) {
              return null;
            }
            const caption =
              segment.attachment.description ||
              segment.attachment.name ||
              `image-${index + 1}`;
            const key = segment.placeholder || `${segment.type}-${index}`;
            if (segment.type === "video") {
              return (
                <VideoPreview
                  key={`task-complete-media-${key}`}
                  src={uri}
                  mimeType={segment.attachment.media_type || "video/mp4"}
                  description={segment.attachment.description}
                  className="w-full sm:w-[220px] lg:w-[260px]"
                  maxHeight="20rem"
                />
              );
            }
            return (
              <ImagePreview
                key={`task-complete-media-${key}`}
                src={uri}
                alt={caption}
                minHeight="12rem"
                maxHeight="20rem"
                className="w-full sm:w-[220px] lg:w-[260px]"
                sizes="(min-width: 1280px) 260px, (min-width: 768px) 220px, 100vw"
                loading={index === 0 ? "eager" : "lazy"}
              />
            );
          })}
        </div>
      )}

      {!streamInProgress && artifactSegments.length > 0 && (
        <div
          className={cn(
            "mt-4",
            hasMultipleArtifacts
              ? "grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3"
              : "space-y-3",
          )}
        >
          {artifactSegments.map((segment, index) => {
            if (!segment.attachment) {
              return null;
            }
            const key = segment.placeholder || `artifact-${index}`;
            return (
              <ArtifactPreviewCard
                key={`task-complete-artifact-${key}`}
                attachment={segment.attachment}
                displayMode={hasMultipleArtifacts ? "title" : undefined}
              />
            );
          })}
        </div>
      )}
    </Card>
  );
}

function getStopReasonCopy(
  stopReason: string | undefined,
  t: ReturnType<typeof useTranslation>,
): StopReasonCopy {
  if (!stopReason) {
    return {
      title: t("events.taskComplete.empty"),
    };
  }

  if (stopReason === "cancelled") {
    return {
      title: t("events.taskComplete.cancelled.title"),
      body: t("events.taskComplete.cancelled.body"),
    };
  }

  return {
    title: t("events.taskComplete.empty"),
    body: t("events.taskComplete.stopReason", { reason: stopReason }),
  };
}
