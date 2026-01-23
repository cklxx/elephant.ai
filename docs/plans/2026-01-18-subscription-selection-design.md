# Subscription Model Selection (Client-Scoped) Design

## Goal
Provide a client-scoped (per browser) model selection flow that can switch between YAML runtime config and CLI subscription models, without writing managed overrides on the server.

## Scope
- Add a subscription catalog service for CLI model discovery.
- Add a per-request LLM selection override that is injected via API and stored in context only.
- Update the LLM indicator to read/write a local selection and include it on task creation.
- Improve mobile legibility of the LLM indicator button.

## Non-goals
- Persisting selection in managed overrides YAML.
- User account level subscription settings.
- Changing YAML config schema.

## Architecture
### New Packages
- `internal/subscription`
  - `CatalogService`: lists models available from CLI subscriptions.
  - `SelectionResolver`: resolves a client selection to provider/model + CLI credentials.

### Context Override
- `internal/agent/app` adds `WithLLMSelection`/`GetLLMSelection` using context keys.
- `ExecutionPreparationService` reads the resolved selection. If present and pinned, it overrides provider/model and bypasses small/vision model switching.

### APIs
- New: `GET /api/internal/subscription/catalog`
  - Returns per-provider model lists (from CLI credentials) with per-provider error details.
- Existing: `GET /api/internal/config/runtime/models`
  - Becomes a thin wrapper around the catalog service for backward compatibility.
- `POST /api/tasks` accepts optional `llm_selection`.

Payload shape (YAML-like):
```yaml
llm_selection:
  mode: cli | yaml
  provider: codex | anthropic | antigravity
  model: gpt-5.2-codex
  source: codex_cli
```

## Data Flow
1. UI loads runtime config snapshot (YAML defaults) and subscription catalog.
2. User picks YAML or a CLI model.
3. UI stores selection in `localStorage` (no secrets).
4. Task creation sends `llm_selection` in the request body.
5. Server resolves selection to CLI credentials and injects it into context.
6. Execution uses resolved provider/model (pinned) for the task.

## Error Handling
- Catalog errors are per-provider and do not block other providers.
- Invalid or missing `llm_selection` falls back to YAML defaults.
- CLI credentials are never returned to the client.

## UX
- LLM indicator shows active source (YAML vs CLI selection).
- Mobile button styling uses higher opacity and clearer outline for legibility.

## Testing
- Backend: catalog service adapters, resolver fallback behavior, API handler injection.
- Frontend: LLM indicator selection persistence and `createTask` payload.
- Manual: select CLI model, run task, confirm model is used; clear selection, confirm YAML model is used.

## Rollout
- No migrations.
- Existing config endpoints remain stable.
