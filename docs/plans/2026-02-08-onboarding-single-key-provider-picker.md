# Plan: Onboarding + Single-Key Provider Picker (CLI/Web/Lark)

Date: 2026-02-08
Owner: cklxx + Codex
Status: In progress

## Context
- Current model selection works via `llm_selection` and subscription catalog, but:
  - CLI still requires manual `<provider>/<model>` input.
  - Web onboarding flow for provider/model is missing.
  - No shared onboarding completion state across surfaces.
- We need first-run onboarding that supports a complete experience with one key (or CLI subscription auth), provider/model selection by picker, and automatic base URL defaults.

## Goals
1. Add a first-run onboarding state store and internal API.
2. Enrich subscription catalog with provider metadata + recommended models.
3. Add CLI `alex setup` flow and support picker-based `alex model use`.
4. Record onboarding completion from CLI/Lark selection.
5. Add web onboarding modal (conversation page) using picker selection + onboarding API.

## Non-goals
- Replacing managed YAML override flow (`alex config set/clear`) for advanced users.
- Multi-device synchronization for onboarding state.

## Deliverables
1. Backend:
   - Provider metadata in `subscription.CatalogProvider`.
   - JSON onboarding state store under config directory.
   - `/api/internal/onboarding/state` GET/PUT endpoints.
2. CLI:
   - `alex setup` command.
   - `alex model use` supports chooser mode (no manual provider/model required).
3. Lark:
   - `/model use` marks onboarding complete.
4. Web:
   - Onboarding modal in conversation page.
   - API client + types for onboarding state.
   - Picker-driven provider/model selection.
5. Tests:
   - Unit tests for store/handler/catalog metadata/CLI flows.

## Progress log
- 2026-02-08 14:25: Plan created.
- 2026-02-08 14:30: Start implementation from latest `main` sync.
- 2026-02-08 14:36: Implemented provider preset metadata in subscription catalog (`display_name`, `auth_mode`, `default_model`, `recommended_models`, `setup_hint`).
- 2026-02-08 14:37: Added onboarding JSON state store + tests and internal onboarding state API handler/routes.
- 2026-02-08 14:39: Added CLI `alex setup`, interactive model picker path, and onboarding completion persistence on model selection.
- 2026-02-08 14:40: Added Lark `/model use` onboarding completion write-through.
- 2026-02-08 14:41: Added web onboarding modal + onboarding API client/types and integrated into conversation page.
- 2026-02-08 14:43: Targeted tests passed:
  - `go test ./internal/app/subscription ./internal/delivery/server/http ./cmd/alex ./internal/delivery/channels/lark`
  - `web` vitest: `runtime-models` + `onboarding-api` tests.
- 2026-02-08 14:44: Full-suite checks attempted; pre-existing failures remain in `make dev-lint` and `make dev-test` (outside this change scope).
