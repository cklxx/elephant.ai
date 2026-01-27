import { AgentCardState } from "./types";

export function getStateColor(state: AgentCardState): string {
  switch (state) {
    case "running":
      return "border-l-blue-500 dark:border-l-blue-400";
    case "completed":
      return "border-l-green-500 dark:border-l-green-400";
    case "failed":
      return "border-l-red-500 dark:border-l-red-400";
    case "cancelled":
      return "border-l-yellow-500 dark:border-l-yellow-400";
    case "idle":
    default:
      return "border-l-muted-foreground/30";
  }
}

export function getStateIcon(state: AgentCardState): string {
  switch (state) {
    case "running":
      return "↻";
    case "completed":
      return "✓";
    case "failed":
      return "✗";
    case "cancelled":
      return "⊘";
    case "idle":
    default:
      return "○";
  }
}

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

export function getStateBadgeColor(state: AgentCardState): string {
  switch (state) {
    case "running":
      return "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400";
    case "completed":
      return "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400";
    case "failed":
      return "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400";
    case "cancelled":
      return "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400";
    case "idle":
    default:
      return "bg-muted text-muted-foreground";
  }
}
