import { useEffect, useMemo, useRef, useState } from "react";

const isTest =
  process.env.NODE_ENV === "test" ||
  process.env.VITEST_WORKER !== undefined;
const TYPEWRITER_CHARS_PER_SECOND = 120;
const TYPEWRITER_MAX_STEP = 1;

export function sliceStreamingContent(content: string, visibleLength: number) {
  const clamped = Math.max(0, Math.min(content.length, visibleLength));
  return content.slice(0, clamped);
}

export function useStreamingAnimation(
  content: string,
  isStreaming: boolean,
  streamFinished: boolean,
) {
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
      const next = Math.min(displayedLengthRef.current + step, targetLength);
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
    ? sliceStreamingContent(normalizedContent, displayedLength)
    : normalizedContent;
  const showStreamingIndicator = isStreaming && !streamFinished;

  return { contentToRender, showStreamingIndicator, shouldAnimate };
}
