"use client";

import { TaskCompleteEvent } from "@/lib/types";
import { formatDuration } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import { MarkdownRenderer } from "@/components/ui/markdown";
import { parseContentSegments, buildAttachmentUri } from "@/lib/attachments";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";

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
  const segments = parseContentSegments(answer, event.attachments);
  const textSegments = segments.filter((segment) => segment.type === "text");
  const textContent = textSegments
    .map((segment) => segment.text ?? "")
    .join("");
  const hasAnswer = textContent.trim().length > 0;
  const mediaSegments = segments.filter(
    (segment) =>
      (segment.type === "image" || segment.type === "video") &&
      segment.attachment,
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
      className="border-l-2 border-green-200 pl-3"
      data-testid="task-complete-event"
    >
      <div className="mt-2 space-y-4">
        {hasAnswer ? (
          <MarkdownRenderer
            content={textContent}
            className="prose prose-slate max-w-none text-sm leading-relaxed text-slate-600"
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
            }}
          />
        ) : (
          <div
            className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm"
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

        {mediaSegments.length > 0 && (
          <div className="flex flex-wrap items-start gap-3">
            {mediaSegments.map((segment, index) => {
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
