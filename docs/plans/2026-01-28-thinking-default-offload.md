# Plan: Default model thinking + offload on next user input (2026-01-28)

## Goal
- Enable model thinking by default, propagate thinking back into the LLM context for the current turn, and offload it when a new user input begins.
- Normalize provider-specific thinking/request/response handling across OpenAI Responses, OpenAI-compatible, Anthropic, and Gemini (antigravity).

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Add typed thinking config/content fields to LLM request/response/message structures and wire defaults in the ReAct engine.
2. Implement provider-specific thinking request + response parsing, plus message conversion for thinking content.
3. Offload thinking content when a new user input is appended; ensure thinking-only responses are retained for intra-turn continuity.
4. Add unit tests for thinking parsing, request payloads, and offload behavior.
5. Run full lint + tests.
6. Commit changes.

## Progress
- 2026-01-28: Plan created; engineering practices reviewed.
- 2026-01-28: Added thinking request/response types, provider parsing, and prompt reinjection helpers; offloaded thinking on new user input.
- 2026-01-28: Added tests for thinking offload, thinking-only message retention, and provider thinking capture.
- 2026-01-28: Ran `./dev.sh lint`.
- 2026-01-28: Ran `./dev.sh test` (passed; linker LC_DYSYMTAB warnings emitted).
