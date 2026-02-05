# Plan: CLI/Lark LLM selection without YAML writes

Date: 2026-02-05
Owner: cklxx + Codex
Status: In progress

## Context
- Web already supports client-scoped subscription model selection via `llm_selection` without mutating managed overrides YAML.
- CLI (`alex model use`) currently persists selection by writing `config.yaml#overrides`.
- Lark currently relies on runtime YAML overrides for long-lived configuration; we want a chat-scoped selection that doesn't touch YAML.

## Goals
- In **CLI** and **Lark** modes, selecting a subscription model should:
  - Persist and be reused automatically on subsequent runs/messages.
  - Avoid writing/rewriting `config.yaml` managed overrides.
  - Resolve credentials at execution time (so OAuth refresh keeps working).

## Non-goals
- Replace the managed overrides system; `alex config` remains YAML-backed.
- Multi-device synchronization for CLI selection.

## Plan
1. Introduce a small, non-YAML state store for LLM selection (JSON), with a configurable path and safe parsing.
2. Update `alex model use|clear` to write/read this selection state instead of `config.yaml#overrides`.
3. Inject resolved `LLMSelection` into execution context for:
   - CLI task runs (stream output + ACP server).
   - Lark gateway tasks (chat/user scoped).
4. Add tests:
   - CLI model command persists without touching YAML.
   - CLI execution path picks up persisted selection.
   - Lark command stores + applies selection (no YAML write).
5. Run full lint + tests.

## Progress log
- 2026-02-05: Plan created; code investigation in progress.
- 2026-02-05: Implemented JSON-backed selection store + CLI/Lark wiring; added regression tests.
