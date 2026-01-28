import { AnyAgentEvent } from "@/lib/types";

export function isSubagentLike(event: AnyAgentEvent): boolean {
  if (!event) return false;

  // Primary: parent_task_id exists and is different from task_id
  // This is the definitive indicator of a subagent
  const parentTask =
    "parent_task_id" in event && typeof event.parent_task_id === "string"
      ? String(event.parent_task_id).trim()
      : "";
  const currentTaskId =
    typeof event.task_id === "string" ? event.task_id.trim() : "";

  // Must have both parent_task_id and task_id, and they must be different
  // This is the ONLY valid indicator of a subagent
  if (parentTask && currentTaskId && parentTask !== currentTaskId) {
    return true;
  }

  // Events without proper parent/child task relationship are NOT subagents
  // even if they have agent_level="subagent" or other markers
  return false;
}
