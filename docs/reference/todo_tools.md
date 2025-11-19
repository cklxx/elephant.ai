# Todo Tool Reference
> Last updated: 2025-11-18


The todo tool suite keeps track of the agent's workstream so that multi-step fixes do not get lost. Use it to capture tasks, track their status, and surface the latest plan back to yourself.

---

## Available Tools

| Tool | Purpose | Typical Usage |
|------|---------|---------------|
| `todo_read` | Returns the current todo list. | Run at the start of an interaction or after any updates to confirm state. |
| `todo_update` | Creates, edits, reorders, or archives todos. | Call whenever you discover new work, complete a task, or want to reprioritise items. |

Both tools are idempotent and expect well-structured JSON payloads. Avoid issuing multiple conflicting updates in a single turn—compose a single `todo_update` call that reflects the desired final state.

---

## Request Structure

### `todo_read`
No arguments are required. The tool responds with an array of todo items and metadata describing each entry.

### `todo_update`
Provide a JSON object with one or more of the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `add` | `[{"content": string, "priority": "low"\|"medium"\|"high"}]` | Append new todos at the end of the list. |
| `update` | `[{"id": string, "status": "pending"\|"in_progress"\|"done", "priority"?: string, "content"?: string}]` | Modify existing todos by id. |
| `remove` | `[id, ...]` | Permanently delete items you no longer need. |
| `reorder` | `[id, ...]` | Re-establish the full ordering of the list. Provide every id exactly once. |

All mutations are applied in the following order: remove → update → add → reorder. If a field is omitted it is treated as no-op.

---

## Usage Guidelines

1. **Read before you write.** Call `todo_read` to understand the current list so you do not duplicate items.
2. **Summarise for the user.** The tool responses can include system reminders that the user should not see. Instead, paraphrase the important parts in your own words.
3. **Keep entries short.** Todo content should capture a single actionable task. Use the description field of your message—not the todo item—for extended reasoning.
4. **Update progress often.** Reflect real status changes (`pending` → `in_progress` → `done`) so future turns start with accurate context.
5. **Archive aggressively.** Remove obsolete or completed tasks once they have been acknowledged in your response.

---

## Example Workflow

```json
// 1. Inspect the current list
{"tool": "todo_read"}

// 2. Add a new task and mark another as in progress
{
  "tool": "todo_update",
  "input": {
    "add": [
      {"content": "Summarise documentation cleanup", "priority": "medium"}
    ],
    "update": [
      {"id": "1", "status": "in_progress"}
    ]
  }
}

// 3. Confirm the result
{"tool": "todo_read"}
```

Follow up with a natural language summary to the user describing the current list and what changed.

---

## Maintenance Notes

- This document replaces the previous raw transcript log with a concise reference.
- Update the request structure tables if the tool schemas change.
- Record nuanced behaviours (e.g., priority semantics) here whenever the runtime is updated.
