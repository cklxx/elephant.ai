/**
 * Hook for managing streaming answer buffer state.
 * Accumulates streaming answer chunks and assistant message deltas
 * for merging into final events.
 */

import { useCallback, useRef } from "react";
import type { AnyAgentEvent, WorkflowNodeOutputDeltaEvent } from "@/lib/types";
import { isWorkflowResultFinalEvent } from "@/lib/typeGuards";
import type { AssistantBufferEntry } from "./types";

export interface UseStreamingAnswerBufferReturn {
  /** Merge streaming task complete event with buffered content */
  mergeStreamingTaskComplete: (event: AnyAgentEvent) => AnyAgentEvent;
  /** Track assistant message from delta event */
  trackAssistantMessage: (event: WorkflowNodeOutputDeltaEvent) => void;
  /** Apply assistant answer fallback to final event */
  applyAssistantAnswerFallback: (event: AnyAgentEvent) => AnyAgentEvent;
  /** Reset streaming answer buffer */
  resetStreamingBuffer: () => void;
  /** Reset assistant message buffer */
  resetAssistantMessageBuffer: () => void;
}

/**
 * Combines two chunks of text, handling overlaps intelligently.
 */
function combineChunks(previous: string, chunk: string): string {
  if (!chunk) return previous;
  if (!previous) return chunk;

  // If new chunk contains all of previous, use new chunk
  if (chunk.startsWith(previous)) {
    return chunk;
  }

  // If previous already ends with chunk, keep previous
  if (previous.endsWith(chunk)) {
    return previous;
  }

  return previous + chunk;
}

export function useStreamingAnswerBuffer(): UseStreamingAnswerBufferReturn {
  const streamingAnswerBufferRef = useRef<Map<string, string>>(new Map());
  const assistantMessageBufferRef = useRef<Map<string, AssistantBufferEntry>>(new Map());

  const resetStreamingBuffer = useCallback(() => {
    streamingAnswerBufferRef.current.clear();
  }, []);

  const resetAssistantMessageBuffer = useCallback(() => {
    assistantMessageBufferRef.current.clear();
  }, []);

  const mergeStreamingTaskComplete = useCallback(
    (event: AnyAgentEvent): AnyAgentEvent => {
      if (!isWorkflowResultFinalEvent(event)) return event;

      const taskId = event.task_id;
      const sessionId = event.session_id ?? "";

      if (!taskId) return event;

      const key = `${sessionId}|${taskId}`;
      const buffer = streamingAnswerBufferRef.current;
      const chunk = event.final_answer ?? "";
      const previous = buffer.get(key) ?? "";
      const isStreaming = event.is_streaming === true;
      const streamFinished = event.stream_finished === true;
      const streamInProgress = isStreaming || event.stream_finished === false;

      if (streamInProgress) {
        const combined = combineChunks(previous, chunk);
        buffer.set(key, combined);
        return {
          ...event,
          final_answer: combined,
        };
      }

      if (streamFinished) {
        const combined = combineChunks(previous, chunk);
        buffer.delete(key);
        return {
          ...event,
          final_answer: combined,
        };
      }

      if (previous) {
        const combined = combineChunks(previous, chunk);
        buffer.delete(key);
        return {
          ...event,
          final_answer: combined,
        };
      }

      return event;
    },
    []
  );

  const trackAssistantMessage = useCallback(
    (event: WorkflowNodeOutputDeltaEvent) => {
      const taskId =
        "task_id" in event && typeof event.task_id === "string"
          ? event.task_id
          : undefined;
      const sessionId =
        "session_id" in event && typeof event.session_id === "string"
          ? event.session_id
          : "";

      if (!taskId) return;

      const iteration =
        typeof event.iteration === "number" ? event.iteration : Number.MIN_SAFE_INTEGER;
      const key = `${sessionId}|${taskId}`;
      const buffer = assistantMessageBufferRef.current;
      const existing = buffer.get(key);

      const baseContent =
        existing && existing.iteration === iteration ? existing.content : "";
      const combined = combineChunks(baseContent, event.delta ?? "");

      buffer.set(key, {
        iteration,
        content: combined,
      });
    },
    []
  );

  const applyAssistantAnswerFallback = useCallback(
    (event: AnyAgentEvent): AnyAgentEvent => {
      if (!isWorkflowResultFinalEvent(event)) return event;

      const taskId =
        "task_id" in event && typeof event.task_id === "string"
          ? event.task_id
          : undefined;
      const sessionId =
        "session_id" in event && typeof event.session_id === "string"
          ? event.session_id
          : "";

      if (!taskId) return event;

      const key = `${sessionId}|${taskId}`;
      const buffer = assistantMessageBufferRef.current;
      const entry = buffer.get(key);
      const existingAnswer =
        typeof event.final_answer === "string" ? event.final_answer : "";
      const streamFinished =
        event.stream_finished === true || event.is_streaming === false;

      if (streamFinished) {
        buffer.delete(key);
      }

      if (existingAnswer.trim().length > 0) {
        return event;
      }

      if (entry && entry.content.trim().length > 0) {
        if (streamFinished) {
          buffer.delete(key);
        }
        return {
          ...event,
          final_answer: entry.content,
        };
      }

      return event;
    },
    []
  );

  return {
    mergeStreamingTaskComplete,
    trackAssistantMessage,
    applyAssistantAnswerFallback,
    resetStreamingBuffer,
    resetAssistantMessageBuffer,
  };
}
