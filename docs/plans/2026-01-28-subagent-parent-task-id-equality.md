# Plan: Analyze subagent events with parent_task_id == task_id (2026-01-28)

## Goal
- Explain why the server can emit events where `agent_level == "subagent"` and `parent_task_id == task_id`, with concrete code-path references.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Trace backend event emission paths that set `agent_level` and task lineage IDs.
2. Identify code paths that can set `parent_task_id` equal to `task_id` for subagent-level events.
3. Summarize findings with file references and conditions.
4. Run full lint + tests.
5. Commit changes.

## Progress
- 2026-01-28: Plan created; engineering practices reviewed.
- 2026-01-28: Traced event emission paths; identified ACP executor envelope fallback that can set parent_task_id == task_id when call.TaskID is empty.
- 2026-01-28: Ran `./dev.sh lint`.
- 2026-01-28: Ran `./dev.sh test` (Go tests passed; linker warnings about malformed LC_DYSYMTAB).
- 2026-01-28: Identified mismatch path: subagent inherits OutputContext from parent, so `prepare` node emits before OutputContext is reset to ensuredTaskID, yielding task_id/parent_task_id equality.
- 2026-01-28: Re-ran `./dev.sh lint`.
- 2026-01-28: Re-ran `./dev.sh test` (Go tests passed; linker warnings about malformed LC_DYSYMTAB).
