"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { ComponentType } from "react";

import type { MarkdownRendererProps } from "@/components/ui/markdown";
import { cn } from "@/lib/utils";
import { LazyMarkdownRenderer } from "./LazyMarkdownRenderer";

const isTest =
  process.env.NODE_ENV === "test" || process.env.VITEST_WORKER !== undefined;
const STREAM_MARKDOWN_BUFFER = 64;
const STREAM_FLUSH_MIN_CHARS = 24;
const TYPEWRITER_CHARS_PER_SECOND = 120;
const TYPEWRITER_MAX_STEP = 1;

function findSafeRenderLength(text: string) {
  if (text.length <= STREAM_MARKDOWN_BUFFER) {
    return text.length;
  }

  const maxIndex = text.length - STREAM_MARKDOWN_BUFFER;
  const newlineIndex = text.lastIndexOf("\n", maxIndex);
  if (newlineIndex >= STREAM_FLUSH_MIN_CHARS) {
    return newlineIndex + 1;
  }

  const spaceIndex = text.lastIndexOf(" ", maxIndex);
  if (spaceIndex >= STREAM_FLUSH_MIN_CHARS) {
    return spaceIndex + 1;
  }

  return maxIndex;
}

export function splitStreamingContent(content: string, visibleLength: number) {
  const visible = content.slice(0, Math.max(0, visibleLength));
  const safeLength = findSafeRenderLength(visible);
  return {
    stable: visible.slice(0, safeLength),
    tail: visible.slice(safeLength),
  };
}

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
  const shouldAnimate = isStreaming && !streamFinished && !isTest;
  const initialLength = shouldAnimate ? 0 : normalizedContent.length;
  const [displayedLength, setDisplayedLength] = useState(initialLength);
  const [targetLength, setTargetLength] = useState(initialLength);
  const displayedLengthRef = useRef(displayedLength);
  const lastContentRef = useRef(normalizedContent);

  useEffect(() => {
    displayedLengthRef.current = displayedLength;
  }, [displayedLength]);

  useEffect(() => {
    if (!shouldAnimate) {
      const fullLength = normalizedContent.length;
      lastContentRef.current = normalizedContent;
      displayedLengthRef.current = fullLength;
      const syncLengths = () => {
        setDisplayedLength(fullLength);
        setTargetLength(fullLength);
      };
      syncLengths();
      return;
    }

    const previous = lastContentRef.current;
    const resetStream = !normalizedContent.startsWith(previous);
    lastContentRef.current = normalizedContent;
    const nextTarget = normalizedContent.length;

    if (resetStream) {
      displayedLengthRef.current = 0;
      const resetLengths = () => {
        setDisplayedLength(0);
        setTargetLength(nextTarget);
      };
      resetLengths();
      return;
    }

    const updateTarget = () => {
      setTargetLength((prev) => Math.max(prev, nextTarget));
    };
    updateTarget();
  }, [normalizedContent, shouldAnimate]);

  useEffect(() => {
    if (!shouldAnimate) {
      return;
    }
    if (displayedLengthRef.current >= targetLength) {
      return;
    }

    let rafId = 0;
    let lastTick = performance.now();
    const tick = (now: number) => {
      const elapsed = now - lastTick;
      lastTick = now;
      const step = Math.max(
        1,
        Math.min(
          TYPEWRITER_MAX_STEP,
          Math.floor((elapsed * TYPEWRITER_CHARS_PER_SECOND) / 1000),
        ),
      );
      const next = Math.min(
        displayedLengthRef.current + step,
        targetLength,
      );
      if (next !== displayedLengthRef.current) {
        displayedLengthRef.current = next;
        setDisplayedLength(next);
      }
      if (next < targetLength) {
        rafId = requestAnimationFrame(tick);
      }
    };

    rafId = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(rafId);
  }, [shouldAnimate, targetLength]);

  const contentToRender = shouldAnimate
    ? splitStreamingContent(normalizedContent, displayedLength)
    : { stable: normalizedContent, tail: "" };
  const showStreamingIndicator = isStreaming && !streamFinished;

  return (
    <div className="space-y-2" aria-live="polite">
      {contentToRender.stable !== "" && (
        <LazyMarkdownRenderer
          content={contentToRender.stable}
          className={className}
          containerClassName={containerClassName}
          components={components}
          attachments={attachments}
          showLineNumbers={showLineNumbers}
        />
      )}
      {shouldAnimate && contentToRender.tail !== "" && (
        <div
          className={cn(
            "whitespace-pre-wrap text-foreground",
            className,
          )}
        >
          {contentToRender.tail}
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
