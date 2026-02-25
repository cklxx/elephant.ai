# LLM Indicator Model Selection Design

## Goal
Enable the LLM indicator chip in the web UI to expose model sources (YAML vs CLI subscription) and allow one-click switching using managed overrides, while leaving YAML configuration untouched.

## Scope
- Web UI: make the indicator clickable and show a dropdown with current model info, YAML option, and CLI subscription model list.
- Backend: add an internal endpoint that lists available models from CLI subscriptions via provider `/models` APIs.
- Config: persist selection via managed overrides (`llm_provider`, `llm_model`, optional `base_url`).

## Non-goals
- Editing YAML runtime config from the UI.
- Enforcing model compatibility beyond listing availability.
- Introducing new provider types or config schema changes.

## UX
- The indicator chip becomes a dropdown trigger.
- Sections:
  - Current: provider/model/auth source (read-only).
  - YAML: a single action to clear overrides and fall back to YAML-sourced values.
  - CLI models: list of models fetched on demand; selecting one applies overrides.
- Loading and error states are shown within the menu; last successful list is cached in memory for fast reopen.

## Backend API
- New endpoint: `GET /api/internal/config/runtime/models` (internal/dev only).
- For each CLI source (codex, antigravity, claude):
  - If a CLI token exists, call `GET {base_url}/models` with provider-appropriate auth headers.
  - Parse a list of model ids from the response.
  - Return per-provider results with `models` and optional `error`.
- No secrets are returned to the client.

## Data Flow
1. UI loads runtime snapshot (existing API).
2. User opens indicator dropdown.
3. UI calls `/api/internal/config/runtime/models` to fetch CLI model list.
4. User selects:
   - YAML: send overrides with `llm_provider`, `llm_model`, `base_url` cleared.
   - CLI model: send overrides for `llm_provider`, `llm_model` (and `base_url` if needed).
5. UI refreshes snapshot and updates chip label/source.

## Error Handling
- Backend errors are scoped per provider and do not block other providers.
- UI shows an inline error for failed providers and keeps YAML selection available.

## Testing
- Backend unit tests for:
  - Handler response shape with mixed success/failure providers.
  - Correct auth headers per provider.
  - Robust parsing of model ids.
- UI tests for:
  - Dropdown renders current status and shows loading/error states.
  - Selecting YAML clears overrides and updates view.

## Rollout
- No migrations.
- Internal-only endpoint guarded by existing internal/dev router logic.
