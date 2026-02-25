import type { ComponentType } from "react";

import { AgentMarkdown } from "@/components/agent/AgentMarkdown";
import { LoadingDots } from "@/components/ui/loading-states";
import type { AttachmentPayload } from "@/lib/types";

import {
  createInlineImageRenderer,
  createInlineMarkdownLink,
  renderInlineAttachment,
  type InlineAttachmentEntry,
} from "./InlineMediaRenderer";
import type { InlineRenderBlock } from "./hooks/useTaskCompleteSegments";

interface StopReasonCopy {
  title: string;
  body?: string;
}

interface TaskCompleteContentProps {
  streamInProgress: boolean;
  streamFinished: boolean;
  showStreamingPlaceholder: boolean;
  shouldRenderMarkdown: boolean;
  contentWithInlineMedia: string;
  inlineAttachments?: Record<string, AttachmentPayload>;
  inlineRenderBlocks: InlineRenderBlock[];
  shouldSoftenSummary: boolean;
  summaryComponents: Record<string, ComponentType<any>>;
  inlineAttachmentMap: Map<string, InlineAttachmentEntry>;
  inlineImageMap: Map<string, string>;
  stopReasonCopy: StopReasonCopy;
  shouldShowFallback: boolean;
  shouldShowAttachmentNotice: boolean;
  attachmentNames: string[];
  emptyCopy: string;
  attachmentsAvailableCopy: string;
  streamingTitle: string;
  streamingHint: string;
}

export function TaskCompleteContent({
  streamInProgress,
  streamFinished,
  showStreamingPlaceholder,
  shouldRenderMarkdown,
  contentWithInlineMedia,
  inlineAttachments,
  inlineRenderBlocks,
  shouldSoftenSummary,
  summaryComponents,
  inlineAttachmentMap,
  inlineImageMap,
  stopReasonCopy,
  shouldShowFallback,
  shouldShowAttachmentNotice,
  attachmentNames,
  emptyCopy,
  attachmentsAvailableCopy,
  streamingTitle,
  streamingHint,
}: TaskCompleteContentProps) {
  const inlineLinkRenderer = createInlineMarkdownLink(inlineAttachmentMap);
  const inlineImageRenderer = createInlineImageRenderer(
    inlineAttachmentMap,
    inlineImageMap,
  );

  return (
    <>
      {showStreamingPlaceholder ? (
        <div
          className="rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-sm"
          data-testid="task-complete-streaming-placeholder"
        >
          <p className="font-medium text-foreground">{streamingTitle}</p>
          <p className="mt-1 inline-flex items-center gap-2 text-muted-foreground">
            <LoadingDots />
            <span>{streamingHint}</span>
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
                a: inlineLinkRenderer,
                img: inlineImageRenderer,
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
            <p className="mt-1 text-muted-foreground">
              {stopReasonCopy.body}
            </p>
          )}
        </div>
      ) : shouldShowAttachmentNotice ? (
        <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-sm">
          <p className="font-medium text-foreground">{emptyCopy}</p>
          <p className="mt-1 text-muted-foreground">{attachmentsAvailableCopy}</p>
          {attachmentNames.length > 0 && (
            <ul className="mt-2 list-disc space-y-1 pl-4 text-xs text-muted-foreground">
              {attachmentNames.map((name) => (
                <li key={name}>{name}</li>
              ))}
            </ul>
          )}
        </div>
      ) : null}
    </>
  );
}
