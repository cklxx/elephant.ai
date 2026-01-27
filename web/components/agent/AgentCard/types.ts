import { AnyAgentEvent, WorkflowToolStartedEvent } from "@/lib/types";

export type AgentCardState =
  | "idle"
  | "running"
  | "completed"
  | "failed"
  | "cancelled";

export interface AgentCardProgress {
  current: number;
  total: number;
  percentage: number;
}

export interface AgentCardStats {
  toolCalls: number;
  tokens: number;
  duration?: number;
}

export interface AgentCardConcurrency {
  index: number;
  total: number;
}

export interface AgentCardData {
  id: string;
  state: AgentCardState;
  type?: string;
  preview?: string;
  description?: string;
  progress?: AgentCardProgress;
  stats: AgentCardStats;
  concurrency?: AgentCardConcurrency;
  events: AnyAgentEvent[];
  statusTone?: "info" | "success" | "warning" | "danger";
}

export interface AgentCardProps {
  data: AgentCardData;
  expanded?: boolean;
  onToggleExpand?: () => void;
  resolvePairedToolStart?: (event: AnyAgentEvent) => WorkflowToolStartedEvent | undefined;
  className?: string;
}
