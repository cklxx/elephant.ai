"use client";

import type { ComponentType } from "react";

import type { MarkdownRendererProps } from "./MarkdownRenderer";
import { cn } from "@/lib/utils";
import { LazyMarkdownRenderer } from "./LazyMarkdownRenderer";
import { useStreamingAnimation } from "./hooks/useStreamingAnimation";

export type StreamingMarkdownRendererProps = MarkdownRendererProps & {
  isStreaming?: boolean;
  streamFinished?: boolean;
  components?: Record<string, ComponentType<any>>;
};

export function StreamingMarkdownRenderer({
  content,
  isStreaming = false,
  streamFinished = false,
  className,
  containerClassName,
  components,
  attachments,
  showLineNumbers,
}: StreamingMarkdownRendererProps) {
  const { contentToRender, showStreamingIndicator, shouldAnimate } =
    useStreamingAnimation(content, isStreaming, streamFinished);
  const shouldRenderMarkdown = !shouldAnimate;

  return (
    <div className="space-y-2" aria-live="polite">
      {contentToRender !== "" && shouldRenderMarkdown && (
        <LazyMarkdownRenderer
          content={contentToRender}
          className={className}
          containerClassName={containerClassName}
          components={components}
          attachments={attachments}
          showLineNumbers={showLineNumbers}
          mode={shouldAnimate ? "streaming" : "static"}
        />
      )}
      {contentToRender !== "" && !shouldRenderMarkdown && (
        <div
          className={cn(
            "whitespace-pre-wrap text-sm leading-relaxed text-foreground",
            className,
          )}
        >
          {contentToRender}
        </div>
      )}
      {showStreamingIndicator && (
        <div
          className={cn(
            "inline-flex items-center gap-2 rounded-full border border-border/60 bg-muted/20 px-3 py-1 text-[11px] font-medium text-muted-foreground",
          )}
          data-testid="markdown-streaming-indicator"
        >
          <span aria-hidden="true">...</span>
          <span className="sr-only">Streaming markdown output</span>
        </div>
      )}
    </div>
  );
}
