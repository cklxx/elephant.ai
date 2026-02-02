# Plan: bg_dispatch task_id removal + main-agent linkage

**Created**: 2026-02-02
**Status**: In Progress
**Author**: cklxx + Codex

## Goals
- Remove user-supplied `task_id` from `bg_dispatch` and error if provided.
- Auto-generate task IDs internally and surface them in tool metadata.
- Ensure background dispatch metadata exposes caller run linkage.
- Refresh external agent docs to match the new contract.

## Plan
- [x] Update `bg_dispatch` tool tests for auto-generated IDs + task_id rejection.
- [x] Update `bg_dispatch` implementation (schema, validation, ID generation, metadata).
- [x] Update external agent docs to remove `task_id` input and explain returned IDs.
- [x] Run full lint + tests.
- [ ] Commit in incremental steps.
