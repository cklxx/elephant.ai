# Plan: Remove think/attention tools

**Created**: 2026-02-01
**Status**: Completed (2026-02-01)
**Author**: cklxx + Codex

## Goals
- Remove `think` and `attention` builtin tools from the registry and codebase.
- Clean up UI/tool presentation paths that assume those tools exist.
- Update tests and documentation references that are tool-specific.
- Keep ReAct phase/"think" workflow semantics intact (only tool removal).

## Scope
- Backend tool definitions, registry wiring, formatting/output categorization.
- Web UI tool presentation for tool calls (icons, labels, filtering).
- Tests that explicitly reference the removed tools.

## Out of scope
- ReAct runtime phases (`think/execute/observe`) and event naming.
- Proactive config `attention` throttling (not the tool).

## Plan
1. Remove tool implementations and registry wiring for `think`/`attention`.
2. Remove formatter/output/UI special-casing for the deleted tools.
3. Update or delete tests that reference those tool names.
4. Run `./dev.sh lint` and `./dev.sh test`.
5. Commit in incremental steps.

## Progress Updates
- 2026-02-01: Plan created.
- 2026-02-01: Removed think/attention tool implementations + registry wiring; cleaned formatter/output/ui handling.
- 2026-02-01: Ran `./dev.sh lint` and `./dev.sh test`.
