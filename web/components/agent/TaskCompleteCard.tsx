"use client";

import { TaskCompleteEvent } from "@/lib/types";
import { formatDuration } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import { MarkdownRenderer } from "@/components/ui/markdown";
import { parseContentSegments, buildAttachmentUri } from "@/lib/attachments";
import { ImagePreview } from "@/components/ui/image-preview";

interface TaskCompleteCardProps {
  event: TaskCompleteEvent;
}

export function TaskCompleteCard({ event }: TaskCompleteCardProps) {
  const t = useTranslation();

  if (!event.final_answer) return null;

  const segments = parseContentSegments(
    event.final_answer,
    event.attachments,
  );
  const textContent = segments
    .filter((segment) => segment.type === "text")
    .map((segment) => segment.text ?? "")
    .join("");
  const imageSegments = segments.filter(
    (segment) => segment.type === "image" && segment.attachment,
  );

  // Build metrics string
  const metrics: string[] = [];
  if (typeof event.total_iterations === "number") {
    metrics.push(`${event.total_iterations} iterations`);
  }
  if (typeof event.total_tokens === "number") {
    metrics.push(`${event.total_tokens} tokens`);
  }
  if (typeof event.duration === "number") {
    metrics.push(formatDuration(event.duration));
  }

  return (
    <div
      className="border-l-2 border-green-200 pl-3"
      data-testid="task-complete-event"
    >
      {event.final_answer && (
        <div className="mt-2">
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
          {imageSegments.length > 0 && (
            <div className="mt-4 grid gap-4 sm:grid-cols-2">
              {imageSegments.map((segment, index) => {
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
                return (
                  <ImagePreview
                    key={`image-segment-${index}`}
                    src={uri}
                    alt={caption}
                    minHeight="12rem"
                    maxHeight="20rem"
                    sizes="(min-width: 1280px) 32vw, (min-width: 768px) 48vw, 100vw"
                  />
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
