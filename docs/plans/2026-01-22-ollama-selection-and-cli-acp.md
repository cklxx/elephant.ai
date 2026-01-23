# Ollama Model Catalog + CLI ACP Executor Disable Plan

**Goal:** Expose Ollama model catalog in the UI with selectable routing, and ensure `acp_executor` is unavailable in CLI tool presets.

## Plan
1) Block `acp_executor` in CLI tool presets and update preset tests.
2) Add Ollama catalog discovery with short timeout, wire it into the subscription catalog response, and extend selection resolver to accept Ollama.
3) Update frontend labels/state to surface Ollama models and handle selection; update tests.
4) Run full lint + tests and record results.

## Progress Log
- 2026-01-22: Plan created.
- 2026-01-22: Blocked `acp_executor` in CLI tool presets and updated preset tests.
- 2026-01-22: Added Ollama catalog discovery + selection support (subscription catalog + selection resolver) and updated UI labels.
- 2026-01-22: Tests run: `./dev.sh lint`, `./dev.sh test` (web tests pass with happy-dom AbortError logs during teardown).
