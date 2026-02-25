import {
  AnyAgentEvent,
  AttachmentPayload,
  WorkflowNodeCompletedEvent,
  WorkflowNodeFailedEvent,
  WorkflowNodeStartedEvent,
  WorkflowResultCancelledEvent,
  WorkflowResultFinalEvent,
  WorkflowToolCompletedEvent,
  WorkflowToolProgressEvent,
  WorkflowToolStartedEvent,
} from "@/lib/types";
import { isEventType } from "@/lib/events/matching";
import {
  isIterationNodeCompletedEvent,
  isIterationNodeStartedEvent,
} from "@/lib/typeGuards";
import { appendBoundedToolStreamChunk } from "@/lib/events/toolStreamBounds";
import type {
  AgentStreamData,
  IterationState,
  NormalizedResearchStep,
  ToolCallState,
} from "@/lib/events/agentStreamTypes";

export type AgentStreamDraft = AgentStreamData;

const syncResearchSteps = (draft: AgentStreamDraft) => {
  draft.researchSteps = draft.stepOrder
    .map((id) => draft.steps.get(id))
    .filter((step): step is NormalizedResearchStep => Boolean(step));
};

const ensureIteration = (
  draft: AgentStreamDraft,
  event: WorkflowNodeStartedEvent,
): IterationState => {
  const iterationNumber = event.iteration!;
  const existing = draft.iterations.get(iterationNumber);
  if (existing) {
    existing.total_iters = event.total_iters ?? existing.total_iters;
    return existing;
  }

  const iteration: IterationState = {
    id: `iteration-${iterationNumber}`,
    iteration: iterationNumber,
    total_iters: event.total_iters,
    status: "running",
    started_at: event.timestamp,
    errors: [],
  };
  draft.iterations.set(iterationNumber, iteration);
  return iteration;
};

const ensureStep = (
  draft: AgentStreamDraft,
  index: number,
  description?: string,
): NormalizedResearchStep => {
  const id = String(index);
  if (!draft.steps.has(id)) {
    const newStep: NormalizedResearchStep = {
      id,
      step_index: index,
      description: description ?? `Step ${index + 1}`,
      status: "planned",
    };
    draft.steps.set(id, newStep);
    if (!draft.stepOrder.includes(id)) {
      draft.stepOrder.push(id);
    }
    return newStep;
  }

  const existing = draft.steps.get(id)!;
  if (description) {
    existing.description = description;
  }
  return existing;
};

const applyStepStarted = (
  draft: AgentStreamDraft,
  event: WorkflowNodeStartedEvent & { step_index: number },
) => {
  const step = ensureStep(draft, event.step_index, event.step_description);
  step.status = "active";
  step.started_at = event.timestamp;
  step.iteration = event.iteration ?? step.iteration;
  step.last_event_at = event.timestamp;
  draft.activeResearchStepId = step.id;
  syncResearchSteps(draft);
};

const applyStepCompleted = (
  draft: AgentStreamDraft,
  event: WorkflowNodeCompletedEvent & { step_index: number },
) => {
  const step = ensureStep(draft, event.step_index, event.step_description);
  step.status = "done";
  step.completed_at = event.timestamp;
  step.result = event.step_result;
  step.iteration = event.iteration ?? step.iteration;
  step.last_event_at = event.timestamp;
  step.error = undefined;
  if (draft.activeResearchStepId === step.id) {
    draft.activeResearchStepId = null;
  }
  syncResearchSteps(draft);
};

const applyWorkflowNodeFailedEvent = (
  draft: AgentStreamDraft,
  event: WorkflowNodeFailedEvent,
) => {
  draft.taskStatus = "error";
  draft.errorMessage = event.error;
  if (draft.currentIteration !== null) {
    const iteration = draft.iterations.get(draft.currentIteration);
    iteration?.errors.push(event.error);
  }
  if (draft.activeResearchStepId) {
    const step = draft.steps.get(draft.activeResearchStepId);
    if (step) {
      step.status = "failed";
      step.error = event.error;
      step.last_event_at = event.timestamp;
    }
  }
  if (draft.activeToolCallId) {
    const toolCall = draft.toolCalls.get(draft.activeToolCallId);
    if (toolCall) {
      toolCall.status = "error";
      toolCall.error = event.error;
      toolCall.completed_at = event.timestamp;
    }
  }
  syncResearchSteps(draft);
};

const applyToolCallStart = (
  draft: AgentStreamDraft,
  event: WorkflowToolStartedEvent,
) => {
  const toolCall: ToolCallState = {
    id: event.call_id,
    call_id: event.call_id,
    tool_name: event.tool_name,
    arguments: event.arguments,
    arguments_preview: event.arguments_preview,
    status: "running",
    stream_chunks: [],
    started_at: event.timestamp,
    iteration: event.iteration,
  };
  draft.toolCalls.set(event.call_id, toolCall);
  draft.activeToolCallId = event.call_id;
};

const applyToolCallStream = (
  draft: AgentStreamDraft,
  event: WorkflowToolProgressEvent,
) => {
  const existing = draft.toolCalls.get(event.call_id);
  if (!existing) return;
  existing.status = "streaming";
  appendBoundedToolStreamChunk(existing.stream_chunks, event.chunk);
  existing.last_stream_at = event.timestamp;
};

const applyToolCallComplete = (
  draft: AgentStreamDraft,
  event: WorkflowToolCompletedEvent,
) => {
  const existing = draft.toolCalls.get(event.call_id);
  if (!existing) {
    draft.toolCalls.set(event.call_id, {
      id: event.call_id,
      call_id: event.call_id,
      tool_name: event.tool_name,
      arguments: {},
      status: event.error ? "error" : "done",
      stream_chunks: [],
      result: event.result,
      error: event.error,
      duration: event.duration,
      started_at: event.timestamp,
      completed_at: event.timestamp,
    });
    return;
  }
  existing.status = event.error ? "error" : "done";
  existing.result = event.result;
  existing.error = event.error;
  existing.duration = event.duration;
  existing.completed_at = event.timestamp;
  draft.activeToolCallId = existing.status === "error" ? existing.call_id : null;
};

const applyIterationComplete = (
  draft: AgentStreamDraft,
  event: WorkflowNodeCompletedEvent,
) => {
  const iterationNumber = event.iteration!;
  const iteration = draft.iterations.get(iterationNumber);
  if (iteration) {
    iteration.status = "done";
    iteration.completed_at = event.timestamp;
    iteration.tokens_used = event.tokens_used;
    iteration.tools_run = event.tools_run;
  }
  if (draft.currentIteration === iterationNumber) {
    draft.currentIteration = null;
  }
};

export const applyEventToDraft = (
  draft: AgentStreamDraft,
  event: AnyAgentEvent,
) => {
  const isIterationStart =
    isIterationNodeStartedEvent(event) &&
    typeof event.iteration === "number" &&
    typeof (event as any).step_index !== "number";

  const isIterationComplete =
    isIterationNodeCompletedEvent(event) &&
    typeof event.iteration === "number" &&
    typeof (event as any).step_index !== "number";

  switch (true) {
    case isIterationStart: {
      const iterationEvent = event as WorkflowNodeStartedEvent & { iteration: number };
      ensureIteration(draft, iterationEvent);
      draft.currentIteration = iterationEvent.iteration;
      draft.taskStatus = "running";
      break;
    }
    case isEventType(event, "workflow.tool.started", "workflow.tool.started"):
      applyToolCallStart(draft, event as WorkflowToolStartedEvent);
      break;
    case isEventType(event, "workflow.tool.progress", "workflow.tool.progress"):
      applyToolCallStream(draft, event as WorkflowToolProgressEvent);
      break;
    case isEventType(event, "workflow.tool.completed", "workflow.tool.completed"):
      applyToolCallComplete(draft, event as WorkflowToolCompletedEvent);
      if (draft.activeToolCallId === (event as WorkflowToolCompletedEvent).call_id) {
        draft.activeToolCallId = null;
      }
      break;
    case isIterationComplete:
      applyIterationComplete(draft, event as WorkflowNodeCompletedEvent);
      draft.totalTokens = (event as WorkflowNodeCompletedEvent).tokens_used ?? draft.totalTokens;
      break;
    case event.event_type === "workflow.input.received": {
      draft.taskStatus = "running";
      draft.finalAnswer = undefined;
      draft.finalAnswerAttachments = undefined;
      draft.errorMessage = undefined;
      draft.totalIterations = undefined;
      draft.totalTokens = undefined;
      draft.currentIteration = null;
      draft.activeToolCallId = null;
      draft.toolCalls.clear();
      draft.iterations.clear();
      draft.steps.clear();
      draft.stepOrder = [];
      draft.researchSteps = [];
      break;
    }
    case isEventType(event, "workflow.result.final", "workflow.result.final"): {
      const complete = event as WorkflowResultFinalEvent;
      const isStreaming = complete.is_streaming === true;
      const streamFinished = complete.stream_finished !== false;

      if (isStreaming && !streamFinished) {
        draft.taskStatus = draft.taskStatus === "idle" ? "running" : draft.taskStatus;
        const prevAnswer = draft.finalAnswer ?? "";
        draft.finalAnswer = prevAnswer + (complete.final_answer ?? "");
      } else {
        draft.taskStatus = "completed";
        const prevAnswer = draft.finalAnswer ?? "";
        const nextAnswer = (() => {
          if (typeof complete.final_answer === "string") {
            return complete.final_answer.length > 0 ? complete.final_answer : prevAnswer;
          }
          return complete.final_answer !== undefined && complete.final_answer !== null
            ? String(complete.final_answer)
            : prevAnswer;
        })();
        draft.finalAnswer = nextAnswer;
      }

      if (complete.attachments !== undefined) {
        draft.finalAnswerAttachments = complete.attachments as
          | Record<string, AttachmentPayload>
          | undefined;
      }
      draft.totalIterations = complete.total_iterations;
      draft.totalTokens = complete.total_tokens;
      draft.currentIteration = null;
      draft.activeToolCallId = null;
      break;
    }
    case isEventType(event, "workflow.result.cancelled", "workflow.result.cancelled"): {
      const cancelled = event as WorkflowResultCancelledEvent;
      draft.taskStatus = "cancelled";
      draft.currentIteration = null;
      draft.activeToolCallId = null;
      draft.errorMessage =
        cancelled.reason && cancelled.reason !== "cancelled" ? cancelled.reason : undefined;
      draft.finalAnswer = undefined;
      draft.finalAnswerAttachments = undefined;
      draft.totalIterations = undefined;
      draft.totalTokens = undefined;
      break;
    }
    case isEventType(event, "workflow.node.failed"):
      applyWorkflowNodeFailedEvent(draft, event as WorkflowNodeFailedEvent);
      break;
    case isEventType(event, "workflow.node.started"):
      if (typeof (event as any).step_index === "number") {
        applyStepStarted(draft, event as WorkflowNodeStartedEvent & { step_index: number });
      }
      break;
    case isEventType(event, "workflow.node.completed"):
      if (typeof (event as any).step_index === "number") {
        applyStepCompleted(draft, event as WorkflowNodeCompletedEvent & { step_index: number });
      }
      break;
    default:
      break;
  }
};
