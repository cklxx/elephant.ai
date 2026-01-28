import { isEventType } from "@/lib/events/matching";
import { AnyAgentEvent } from "@/lib/types";

export function isSubagentLike(event: AnyAgentEvent): boolean {
  if (!event) return false;

  if (event.agent_level === "subagent") return true;
  const unknownEvent = event as {
    is_subtask?: unknown;
    parent_task_id?: unknown;
    node_id?: unknown;
    call_id?: unknown;
  };
  if ("is_subtask" in event && Boolean(unknownEvent.is_subtask)) {
    return true;
  }

  const parentTask =
    "parent_task_id" in event && typeof unknownEvent.parent_task_id === "string"
      ? String(unknownEvent.parent_task_id).trim()
      : "";
  const currentTaskId =
    typeof event.task_id === "string" ? event.task_id.trim() : "";
  if (parentTask && parentTask !== currentTaskId) return true;

  const nodeId =
    "node_id" in event && typeof unknownEvent.node_id === "string"
      ? String(unknownEvent.node_id).toLowerCase()
      : "";
  if (nodeId.startsWith("subagent") || nodeId.startsWith("subflow-")) {
    return true;
  }

  const callId =
    "call_id" in event && typeof unknownEvent.call_id === "string"
      ? String(unknownEvent.call_id).toLowerCase()
      : "";
  if (callId.startsWith("subagent")) {
    return true;
  }

  const taskId =
    typeof event.task_id === "string" ? event.task_id.toLowerCase() : "";
  if (taskId.startsWith("subagent")) {
    return true;
  }

  return isEventType(
    event,
    "workflow.subflow.progress",
    "workflow.subflow.completed",
  );
}
