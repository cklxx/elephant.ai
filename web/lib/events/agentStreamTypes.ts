import type { AttachmentPayload } from "@/lib/types";
import { EventLRUCache } from "@/lib/eventAggregation";

export type ToolCallStatus = "pending" | "running" | "streaming" | "done" | "error";

export type UnifiedStepStatus = "planned" | "active" | "done" | "failed";

export interface ToolCallState {
  id: string;
  call_id: string;
  tool_name: string;
  arguments: Record<string, any>;
  arguments_preview?: string;
  status: ToolCallStatus;
  stream_chunks: string[];
  result?: unknown;
  error?: string;
  duration?: number;
  started_at: string;
  completed_at?: string;
  last_stream_at?: string;
  iteration?: number;
}

export interface IterationState {
  id: string;
  iteration: number;
  total_iters?: number;
  status: "running" | "done";
  started_at: string;
  completed_at?: string;
  tokens_used?: number;
  tools_run?: number;
  errors: string[];
}

export interface NormalizedResearchStep {
  id: string;
  step_index: number;
  description: string;
  status: UnifiedStepStatus;
  started_at?: string;
  completed_at?: string;
  result?: string;
  iteration?: number;
  last_event_at?: string;
  error?: string;
}

export interface AgentStreamData {
  eventCache: EventLRUCache;
  toolCalls: Map<string, ToolCallState>;
  iterations: Map<number, IterationState>;
  steps: Map<string, NormalizedResearchStep>;
  stepOrder: string[];
  researchSteps: NormalizedResearchStep[];
  currentIteration: number | null;
  activeToolCallId: string | null;
  activeResearchStepId: string | null;
  taskStatus: "idle" | "running" | "completed" | "cancelled" | "error";
  finalAnswer?: string;
  finalAnswerAttachments?: Record<string, AttachmentPayload>;
  totalIterations?: number;
  totalTokens?: number;
  errorMessage?: string;
}
