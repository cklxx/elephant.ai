# Subagent event and payload best practices

This note summarizes the UI/UX conventions we want for delegated subagent timelines and what the backend should emit to keep the experience reliable.

## Frontend presentation principles
- **Threaded context header:** Group subagent activity by `parent_task_id`/`subtask_index` so users see a single header that anchors all iterations for that delegate. Keep concise labels such as `Subagent Task 1/3` and surface the latest preview or goal text.
- **Inline progress + health pills:** Every subagent block should render progress (`Progress 2/4`) and quick stats (`3 tool calls · 1200 tokens`) so users can scan for stuck delegates without opening tool outputs.
- **Status first, details second:** Completion summaries should highlight success/failed counts in a high-contrast pill, with rich details (tool output cards, final answers) stacked below.
- **Stable ordering:** Use the first-seen order of subagents and deterministic keys (`parent_task_id` + `subtask_index` or `call_id`) to avoid jumping headers as streams arrive out of order.
- **Carry forward hints:** Once a subagent shares `subtask_preview` or `max_parallel`, persist that label across subsequent tool and summary events to avoid flicker.

## What the server should send
- **Explicit subagent identity:** Always include `agent_level: "subagent"`, `parent_task_id`, and `subtask_index` for delegated work. If the call is initiated via a tool, prefix `call_id` with `subagent:` for resilience.
- **Planning metadata:** Provide `total_subtasks` and optional `subtask_preview` strings so the UI can render scoped titles and tooltips without guessing.
- **Concurrency hints:** Emit `max_parallel` when multiple delegates are spun up; the UI shows this as a pill next to the header.
- **Progress snapshots:** Send `subagent_progress` events with `completed`, `total`, `tokens`, and `tool_calls`. The frontend uses these to render the inline progress and stats pills without recomputing on the client.
- **Completion rollups:** Send `subagent_complete` events with `success`, `failed`, `tokens`, and `tool_calls` to clearly mark the end state. Avoid only emitting per-tool/task records; the rollup keeps the timeline readable.
- **Consistency across tool events:** Tool start/stream/complete events for subagents should carry the same identity fields and, when possible, the running totals for tokens/tool calls to keep pills accurate.

## Example payloads
```json
{
  "event_type": "subagent_progress",
  "agent_level": "subagent",
  "session_id": "abc",
  "task_id": "delegate-task-1",
  "parent_task_id": "root-task-9",
  "subtask_index": 0,
  "total_subtasks": 3,
  "subtask_preview": "Audit console UI for nested delegates",
  "max_parallel": 2,
  "completed": 1,
  "total": 3,
  "tokens": 820,
  "tool_calls": 2
}
```

```json
{
  "event_type": "subagent_complete",
  "agent_level": "subagent",
  "session_id": "abc",
  "task_id": "delegate-task-1",
  "parent_task_id": "root-task-9",
  "subtask_index": 0,
  "total_subtasks": 3,
  "success": 3,
  "failed": 0,
  "tokens": 4210,
  "tool_calls": 6
}
```
