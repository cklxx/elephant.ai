# Plan: Single-Provider Key Onboarding for CLI/Web + Lark Bot Helper (2026-02-12)

## Summary
- Target: let beginner users complete first-run setup with a single API key for one provider (`kimi`/`glm`/`minimax`/`openrouter`/`openai`/`anthropic`), with default latest model and optional model selection.
- Scope:
  - Subscription catalog exposes manual providers + key creation links.
  - Runtime supports provider aliases (`kimi`/`glm`/`minimax`) without unknown-provider failures.
  - `alex setup` supports provider + api key path (non-DB dependency).
  - Web onboarding supports provider/model/api-key setup and provider-specific key links.
  - Lark bot setup helper links + copyable template in CLI/Web onboarding surfaces.
- Keep Lark runtime dependency file/memory based; no DB requirement introduced.

## Engineering Notes (best-practice anchors)
- Keep product provider IDs decoupled from protocol adapter routing (OpenAI-compatible vs Anthropic vs Codex-family), following adapter-boundary conventions.
- Prefer explicit, validated defaults and deterministic fallbacks (default model, base URL, selection source).
- Preserve backward compatibility on existing CLI subscription-based flow.

## Work Items
- [x] A. Expand provider metadata + catalog payload shape (`key_create_url`) and include manual providers.
- [x] B. Add provider alias routing in LLM factory/runtime path for `kimi`/`glm`/`minimax`.
- [x] C. Extend `alex setup` for single-key config flow (provider/model/api-key, defaults, lark helper links).
- [x] D. Update web onboarding modal for provider/model/api-key + key-link + lark bot helper UX.
- [x] E. Add/update tests (Go + web).
- [x] F. Full validation (lint + tests), mandatory code review skill flow, incremental commits.

## Progress Log
- 2026-02-12 15:35 CST: Plan created; repository conventions + related implementation surfaces audited.
- 2026-02-12 15:45 CST: Backend catalog + provider metadata + runtime alias support landed; CLI setup single-key flow and lark helper implemented.
- 2026-02-12 15:50 CST: Web onboarding updated with provider key link, API key input, runtime override save, and Feishu helper copy/open actions.
- 2026-02-12 15:54 CST: Validation completed (`go test ./...`, `./scripts/run-golangci-lint.sh run ./...`, `npm --prefix web run lint`, `npm --prefix web test`).
