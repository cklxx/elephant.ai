"use client";

import { useMemo } from "react";
import type { ComponentType } from "react";

import type { MarkdownRendererProps } from "@/components/ui/markdown";
import { cn } from "@/lib/utils";
import { StreamingIndicator } from "@/components/ui/loading-states";
import { LazyMarkdownRenderer } from "./LazyMarkdownRenderer";

type StreamingMarkdownRendererProps = MarkdownRendererProps & {
  /**
   * When true, show a lightweight streaming affordance beneath the markdown.
   * This should be driven by the event's streaming flags so users can see the
   * final answer arrive incrementally.
   */
  isStreaming?: boolean;
  /**
   * Optional flag to explicitly mark the stream as finished. This hides the
   * streaming indicator even if `isStreaming` was previously true.
   */
  streamFinished?: boolean;
  /**
   * Optional override for markdown components; kept here so callers don't have
   * to import types from the base renderer.
   */
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
  const normalizedContent = useMemo(() => content ?? "", [content]);
  const showStreamingIndicator = isStreaming && !streamFinished;

  return (
    <div className="space-y-2" aria-live="polite">
      <LazyMarkdownRenderer
        content={normalizedContent}
        className={className}
        containerClassName={containerClassName}
        components={components}
        attachments={attachments}
        showLineNumbers={showLineNumbers}
      />
      {showStreamingIndicator && (
        <div
          className={cn(
            "inline-flex items-center gap-2 rounded-full border border-border/60 bg-muted/20 px-3 py-1 text-[11px] font-medium text-muted-foreground",
          )}
          data-testid="markdown-streaming-indicator"
        >
          <StreamingIndicator />
          <span className="sr-only">Streaming markdown output</span>
        </div>
      )}
    </div>
  );
}
