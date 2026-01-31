# Plan: Remove think/attention tool references

**Created**: 2026-02-01
**Status**: Completed (2026-02-01)
**Author**: cklxx + Codex

## Goals
- Remove documentation/references that treat `think` or `attention` as tools.
- Keep ReAct phase semantics and proactive attention config intact.

## Scope
- Docs/plan/reference files that list builtin tools or describe tool behavior.

## Out of scope
- ReAct phase naming (think/execute/observe).
- Proactive attention config and throttling behavior.

## Plan
1. Locate remaining references to think/attention as tools.
2. Update docs to remove tool-specific mentions while preserving phase semantics.
3. Run `./dev.sh lint` and `./dev.sh test`.
4. Commit changes.

## Progress Updates
- 2026-02-01: Plan created.
- 2026-02-01: Removed legacy think/attention tool mentions from historical plan docs.
