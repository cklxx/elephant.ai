"use client";

import { useMemo } from "react";

import { WorkflowResultFinalEvent } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";
import {
  parseContentSegments,
  buildAttachmentUri,
  replacePlaceholdersWithMarkdown,
  getAttachmentSegmentType,
  ContentSegment,
} from "@/lib/attachments";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "./ArtifactPreviewCard";
import { Card, CardContent } from "@/components/ui/card";
import { StreamingMarkdownRenderer } from "./StreamingMarkdownRenderer";
import { LoadingDots } from "@/components/ui/loading-states";

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
  const streamInProgress =
    event.stream_finished === false ||
    (event.is_streaming === true && event.stream_finished !== true);
  const streamFinished = event.stream_finished === true;
  const inlineAttachments = streamInProgress ? undefined : attachments;
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
  const { inlineAttachmentMap, attachmentNames, hasAttachments } = useMemo(() => {
    if (!inlineAttachments) {
      return {
        inlineAttachmentMap: new Map<
          string,
          {
            key: string;
            type: string;
            description?: string;
            mime?: string;
            attachment: NonNullable<WorkflowResultFinalEvent["attachments"]>[string];
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
        attachment: NonNullable<WorkflowResultFinalEvent["attachments"]>[string];
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
    const segments = parseContentSegments(markdownAnswer, attachments);
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
  }, [markdownAnswer, attachments]);
  const hasAnswerContent = contentWithInlineMedia.trim().length > 0;
  const shouldRenderMarkdown =
    hasAnswerContent || streamInProgress || (streamFinished && event.stop_reason !== "cancelled");
  const hasUnrenderedAttachments =
    unreferencedMediaSegments.length > 0 || artifactSegments.length > 0;
  const shouldShowFallback = !shouldRenderMarkdown && !hasUnrenderedAttachments && !hasAttachments;
  const shouldShowAttachmentNotice =
    !shouldRenderMarkdown && !hasUnrenderedAttachments && hasAttachments;

  const stopReasonCopy = getStopReasonCopy(event.stop_reason, t);

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
      />
    );
  };

  return (
    <Card data-testid="task-complete-event">
      <CardContent className="mt-2 space-y-4 p-4">
        {streamInProgress && !hasAnswerContent ? (
          <div
            className="rounded-md border border-slate-100 bg-slate-50/70 px-3 py-2 text-sm"
            data-testid="task-complete-streaming-placeholder"
          >
            <p className="font-medium text-slate-700">
              {t("events.taskComplete.streaming")}
            </p>
            <p className="mt-1 inline-flex items-center gap-2 text-slate-500">
              <LoadingDots />
              <span>{t("events.taskComplete.streamingHint")}</span>
            </p>
          </div>
        ) : shouldRenderMarkdown ? (
          <>
            <StreamingMarkdownRenderer
              content={contentWithInlineMedia}
              className="prose prose-slate max-w-none text-sm leading-relaxed text-slate-900"
              attachments={inlineAttachments}
              isStreaming={streamInProgress}
              streamFinished={streamFinished}
              components={{
                code: ({ inline, className, children, ...props }: any) => {
                  if (inline) {
                    return (
                      <code
                        className="whitespace-nowrap rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-800"
                        {...props}
                      >
                        {children}
                      </code>
                    );
                  }
                  return (
                    <code
                      className="block overflow-x-auto rounded-md border border-slate-200 bg-slate-50 p-4 font-mono text-xs leading-relaxed text-slate-800"
                      {...props}
                    >
                      {children}
                    </code>
                  );
                },
                pre: ({ children }: any) => (
                  <div className="my-4">{children}</div>
                ),
                p: ({ children }: any) => (
                  <div className="mb-4 leading-relaxed text-slate-900">
                    {children}
                  </div>
                ),
                ul: ({ children }: any) => (
                  <ul className="mb-4 space-y-2 leading-relaxed text-slate-900">
                    {children}
                  </ul>
                ),
                ol: ({ children }: any) => (
                  <ol className="mb-4 space-y-2 leading-relaxed text-slate-900">
                    {children}
                  </ol>
                ),
                li: ({ children }: any) => (
                  <li className="leading-relaxed text-slate-900">{children}</li>
                ),
                strong: ({ children }: any) => (
                  <strong className="font-bold text-slate-900">{children}</strong>
                ),
                img: ({ src, alt }: { src?: string; alt?: string }) => {
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
                        description={matchedAttachment.description || alt || matchedAttachment.key}
                        className="w-full"
                        maxHeight="20rem"
                      />
                    );
                  }

                  if (matchedAttachment && (matchedAttachment.type === "document" || matchedAttachment.type === "embed")) {
                    return (
                      <div className="my-4">
                        <ArtifactPreviewCard attachment={matchedAttachment.attachment} />
                      </div>
                    );
                  }

                  return <InlineMarkdownImage src={recoveredSrc} alt={alt} />;
                },
              }}
            />
            <div
              className="h-px w-full bg-border/70"
              aria-hidden="true"
              data-testid="task-complete-answer-divider"
            />
          </>
        ) : shouldShowFallback ? (
          <div
            className="rounded-md border border-slate-100 bg-slate-50/70 px-3 py-2 text-sm"
            data-testid="task-complete-fallback"
          >
            <p className="font-medium text-slate-700">{stopReasonCopy.title}</p>
            {stopReasonCopy.body && (
              <p className="mt-1 text-slate-500">{stopReasonCopy.body}</p>
            )}
          </div>
        ) : shouldShowAttachmentNotice ? (
          <div className="rounded-md border border-slate-100 bg-slate-50/70 px-3 py-2 text-sm">
            <p className="font-medium text-slate-700">
              {t("events.taskComplete.empty")}
            </p>
            <p className="mt-1 text-slate-500">
              {t("events.taskComplete.attachmentsAvailable")}
            </p>
            {attachmentNames.length > 0 && (
              <ul className="mt-2 list-disc space-y-1 pl-4 text-xs text-slate-600">
                {attachmentNames.map((name) => (
                  <li key={name}>{name}</li>
                ))}
              </ul>
            )}
          </div>
        ) : null}

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
                />
              );
            })}
          </div>
        )}

        {!streamInProgress && artifactSegments.length > 0 && (
          <div className="mt-4 space-y-3">
            {artifactSegments.map((segment, index) => {
              if (!segment.attachment) {
                return null;
              }
              const key = segment.placeholder || `artifact-${index}`;
              return (
                <ArtifactPreviewCard
                  key={`task-complete-artifact-${key}`}
                  attachment={segment.attachment}
                />
              );
            })}
          </div>
        )}
      </CardContent>
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
