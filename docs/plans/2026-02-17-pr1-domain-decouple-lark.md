# PR-1: Domain Decouple from Lark Preset/Prompt

Created: 2026-02-17
Owner: cklxx
Status: In Progress

## Objective

Remove Lark-specific semantics from `internal/domain/agent/presets/*` so domain remains channel-agnostic.

## Scope

1. Remove `ToolPresetLarkLocal` from domain tool presets.
2. Remove `lark_*` and explicit “Lark flows” wording from domain prompt suffix.
3. Keep Lark-specific defaults in delivery layer (`bootstrap`), not in domain constants.
4. Update affected tests.

## Progress

- [x] Locate domain leaks and caller paths
- [x] Implement domain preset/prompt cleanup
- [x] Update tests
- [x] Run targeted tests + lint
- [x] Summarize findings and next PR handoff

## Findings

- Domain preset constants are now channel-agnostic (`full/read-only/safe/architect` only).
- Lark-specific routing language was removed from shared domain prompt guardrails.
- Lark delivery default now uses `tool_preset: full` from bootstrap.
- Added compatibility regression coverage: legacy `lark-local` values fallback to `full` in preset resolver.
