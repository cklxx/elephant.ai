"use client";

import { useMemo } from "react";

import { TaskCompleteEvent } from "@/lib/types";
import { formatDuration } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import { MarkdownImage, MarkdownRenderer } from "@/components/ui/markdown";
import {
  parseContentSegments,
  buildAttachmentUri,
  replacePlaceholdersWithMarkdown,
  getAttachmentSegmentType,
} from "@/lib/attachments";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "./ArtifactPreviewCard";

interface StopReasonCopy {
  title: string;
  body?: string;
}

interface TaskCompleteCardProps {
  event: TaskCompleteEvent;
}

export function TaskCompleteCard({ event }: TaskCompleteCardProps) {
  const t = useTranslation();
  const answer = event.final_answer ?? "";
  const contentWithInlineMedia = replacePlaceholdersWithMarkdown(
    answer,
    event.attachments,
  );
  const inlineAttachmentMap = useMemo(() => {
    if (!event.attachments) {
      return new Map<
        string,
        {
          key: string;
          type: string;
          description?: string;
          mime?: string;
          attachment: NonNullable<TaskCompleteEvent["attachments"]>[string];
        }
      >();
    }

    return Object.entries(event.attachments).reduce(
      (acc, [key, attachment]) => {
        const uri = buildAttachmentUri(attachment);
        if (!uri) {
          return acc;
        }
        acc.set(uri, {
          key,
          type: getAttachmentSegmentType(attachment),
          description: attachment.description,
          mime: attachment.media_type,
          attachment,
        });
        return acc;
      },
      new Map<
        string,
        {
          key: string;
          type: string;
          description?: string;
          mime?: string;
          attachment: NonNullable<TaskCompleteEvent["attachments"]>[string];
        }
      >(),
    );
  }, [event.attachments]);
  const segments = parseContentSegments(answer, event.attachments);
  const referencedPlaceholders = new Set(
    Array.from(answer.matchAll(/\[[^\[\]]+\]/g)).map((match) => match[0]),
  );
  const hasAnswer = contentWithInlineMedia.trim().length > 0;

  const unreferencedMediaSegments = segments.filter(
    (segment) =>
      (segment.type === "image" || segment.type === "video") &&
      segment.attachment &&
      (!segment.placeholder || !referencedPlaceholders.has(segment.placeholder)),
  );
  const artifactSegments = segments.filter(
    (segment) =>
      (segment.type === "document" || segment.type === "embed") &&
      segment.attachment &&
      (!segment.placeholder || !referencedPlaceholders.has(segment.placeholder)),
  );

  const stopReasonCopy = getStopReasonCopy(event.stop_reason, t);

  // Build metrics string
  const metrics: string[] = [];
  if (typeof event.total_iterations === "number") {
    metrics.push(t("events.taskComplete.metrics.iterations", { count: event.total_iterations }));
  }
  if (typeof event.total_tokens === "number") {
    metrics.push(t("events.taskComplete.metrics.tokens", { count: event.total_tokens }));
  }
  if (typeof event.duration === "number") {
    metrics.push(t("events.taskComplete.metrics.duration", { duration: formatDuration(event.duration) }));
  }

  return (
    <div
      className="border-l border-green-100/80 bg-green-50/30 pl-3"
      data-testid="task-complete-event"
    >
      <div className="mt-2 space-y-4">
        {hasAnswer ? (
          <MarkdownRenderer
            content={contentWithInlineMedia}
            className="prose prose-slate max-w-none text-sm leading-relaxed text-slate-600"
            attachments={event.attachments}
            components={{
              code: ({ inline, className, children, ...props }: any) => {
                if (inline) {
                  return (
                    <code
                      className="whitespace-nowrap rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-600"
                      {...props}
                    >
                      {children}
                    </code>
                  );
                }
                return (
                  <code
                    className="block overflow-x-auto rounded-md border border-slate-200 bg-slate-50 p-4 font-mono text-xs leading-relaxed text-slate-600"
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
                <p className="mb-4 leading-relaxed text-slate-600">
                  {children}
                </p>
              ),
              ul: ({ children }: any) => (
                <ul className="mb-4 space-y-2 leading-relaxed text-slate-600">
                  {children}
                </ul>
              ),
              ol: ({ children }: any) => (
                <ol className="mb-4 space-y-2 leading-relaxed text-slate-600">
                  {children}
                </ol>
              ),
              li: ({ children }: any) => (
                <li className="leading-relaxed text-slate-600">{children}</li>
              ),
              strong: ({ children }: any) => (
                <strong className="font-bold text-slate-600">{children}</strong>
              ),
              img: ({ src, alt, ...imgProps }: { src?: string; alt?: string }) => {
                if (!src) {
                  return null;
                }
                const matchedAttachment = inlineAttachmentMap.get(src);
                if (matchedAttachment?.type === "video") {
                  return (
                    <VideoPreview
                      key={`task-complete-inline-video-${matchedAttachment.key}`}
                      src={src}
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

                return (
                  <MarkdownImage
                    src={src}
                    alt={alt}
                    className="my-4 max-h-80 w-full object-contain"
                    {...imgProps}
                  />
                );
              },
            }}
          />
        ) : (
          <div
            className="rounded-md border border-slate-100 bg-slate-50/70 px-3 py-2 text-sm"
            data-testid="task-complete-fallback"
          >
            <p className="font-medium text-slate-700">{stopReasonCopy.title}</p>
            {stopReasonCopy.body && (
              <p className="mt-1 text-slate-500">{stopReasonCopy.body}</p>
            )}
          </div>
        )}

        {metrics.length > 0 && (
          <div
            className="text-xs uppercase tracking-wide text-slate-400"
            data-testid="task-complete-metrics"
          >
            {metrics.join(" · ")}
          </div>
        )}

        {unreferencedMediaSegments.length > 0 && (
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

        {artifactSegments.length > 0 && (
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
      </div>
    </div>
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
