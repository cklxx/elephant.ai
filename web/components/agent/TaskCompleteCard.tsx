"use client";

import Image from "next/image";
import { TaskCompleteEvent } from "@/lib/types";
import { formatDuration } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import { MarkdownRenderer } from "@/components/ui/markdown";
import {
  replacePlaceholdersWithMarkdown,
  buildAttachmentUri,
} from "@/lib/attachments";

interface TaskCompleteCardProps {
  event: TaskCompleteEvent;
}

export function TaskCompleteCard({ event }: TaskCompleteCardProps) {
  const t = useTranslation();

  if (!event.final_answer) return null;

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
            content={replacePlaceholdersWithMarkdown(
              event.final_answer,
              event.attachments,
            )}
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
        </div>
      )}

      {event.attachments && Object.keys(event.attachments).length > 0 && (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          {Object.entries(event.attachments).map(([key, attachment]) => {
            const uri = buildAttachmentUri(attachment);
            if (!uri) {
              return null;
            }
            return (
              <figure
                key={key}
                className="rounded-lg border border-slate-200 bg-white p-2"
              >
                <div
                  className="relative w-full overflow-hidden rounded bg-slate-50"
                  style={{ minHeight: "10rem", maxHeight: "18rem" }}
                >
                  <Image
                    src={uri}
                    alt={attachment.description || attachment.name || key}
                    fill
                    className="object-contain"
                    sizes="(min-width: 1024px) 33vw, (min-width: 768px) 50vw, 100vw"
                    unoptimized
                  />
                </div>
                <figcaption className="mt-2 text-[11px] uppercase tracking-wide text-slate-500">
                  {attachment.description || attachment.name || key}
                </figcaption>
              </figure>
            );
          })}
        </div>
      )}
    </div>
  );
}
