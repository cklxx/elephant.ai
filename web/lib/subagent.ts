import { AnyAgentEvent } from "@/lib/types";

export function isSubagentLike(event: AnyAgentEvent): boolean {
  if (!event) return false;

  // Primary: parent_run_id exists and is different from run_id
  // This is the definitive indicator of a subagent
  const parentRun =
    "parent_run_id" in event && typeof event.parent_run_id === "string"
      ? String(event.parent_run_id).trim()
      : "";
  const currentRunId =
    typeof event.run_id === "string" ? event.run_id.trim() : "";

  // Must have both parent_run_id and run_id, and they must be different
  // This is the ONLY valid indicator of a subagent
  if (parentRun && currentRunId && parentRun !== currentRunId) {
    return true;
  }

  // Events without proper parent/child run relationship are NOT subagents
  // even if they have agent_level="subagent" or other markers
  return false;
}
