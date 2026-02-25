import { Loader2, Check, X, Ban, Circle } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { AgentCardState } from "./types";

export function getStateLabel(state: AgentCardState): string {
  switch (state) {
    case "running":
      return "Running";
    case "completed":
      return "Completed";
    case "failed":
      return "Failed";
    case "cancelled":
      return "Cancelled";
    case "idle":
      return "Idle";
    default:
      return "Unknown";
  }
}

export function getStateAccentColor(state: AgentCardState): string {
  switch (state) {
    case "running":
      return "bg-blue-500";
    case "completed":
      return "bg-emerald-500";
    case "failed":
      return "bg-red-500";
    case "cancelled":
      return "bg-amber-500";
    case "idle":
    default:
      return "bg-muted-foreground/30";
  }
}

export function getStateContainerStyle(state: AgentCardState): string {
  switch (state) {
    case "running":
      return "bg-blue-50/30 dark:bg-blue-950/20";
    case "failed":
      return "bg-red-50/30 dark:bg-red-950/20";
    default:
      return "bg-muted/10";
  }
}

export function getStateIconContainerStyle(state: AgentCardState): string {
  switch (state) {
    case "running":
      return "bg-blue-100/60 dark:bg-blue-900/40";
    case "completed":
      return "bg-emerald-100/60 dark:bg-emerald-900/40";
    case "failed":
      return "bg-red-100/60 dark:bg-red-900/40";
    case "cancelled":
      return "bg-amber-100/60 dark:bg-amber-900/40";
    case "idle":
    default:
      return "bg-muted/40";
  }
}

export function getStateIconColor(state: AgentCardState): string {
  switch (state) {
    case "running":
      return "text-blue-600 dark:text-blue-400";
    case "completed":
      return "text-emerald-600 dark:text-emerald-400";
    case "failed":
      return "text-red-600 dark:text-red-400";
    case "cancelled":
      return "text-amber-600 dark:text-amber-400";
    case "idle":
    default:
      return "text-muted-foreground";
  }
}

export function getStateLucideIcon(state: AgentCardState): LucideIcon {
  switch (state) {
    case "running":
      return Loader2;
    case "completed":
      return Check;
    case "failed":
      return X;
    case "cancelled":
      return Ban;
    case "idle":
    default:
      return Circle;
  }
}
