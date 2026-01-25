# Plan: Tool Schema Hardening for LLM Requests (2026-01-25)

## Goal
- Prevent invalid tool schema payloads (e.g., null `parameters` / `properties`) from reaching LLM providers.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Normalize tool parameter schemas before serialization (ensure `type: object` + `properties: {}` when missing).
2. Apply normalization in OpenAI/Codex/Anthropic/Antigravity tool conversions.
3. Add tests that assert serialized schemas never produce null `parameters` / `properties`.
4. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Normalized tool schemas in LLM tool conversions and added tests.
- 2026-01-25: Ran `make fmt`, `make vet`, `make test`.
