# Plan: Remove DB, Sandbox, and Auth from elephant.ai

## Context

elephant.ai is moving to a local-first open-source model. Three subsystems are unnecessary for local operation and add complexity:
1. **PostgreSQL** — all Postgres stores (session, task, kernel, event history, lark, auth)
2. **Sandbox** — Docker-based remote tool execution (only needed for hosted web mode)
3. **Auth/User Login** — user registration, JWT, OAuth, middleware (irrelevant for local/single-user)

File-based and in-memory fallbacks already exist for most stores. The system gracefully degrades without DB today.

---

## Batch 1: Remove Lark Postgres Stores

**Delete files (7):**
- `internal/infra/lark/oauth/token_store_postgres.go`
- `internal/infra/lark/oauth/token_store_postgres_test.go`
- `internal/delivery/channels/lark/task_store_postgres.go`
- `internal/delivery/channels/lark/chat_session_binding_postgres.go`
- `internal/delivery/channels/lark/chat_session_binding_store_test.go`
- `internal/delivery/channels/lark/plan_review_postgres.go`
- `internal/delivery/channels/lark/plan_review_store_test.go`

**Edit (1):**
- `internal/domain/task/store.go` — update comment referencing `Lark TaskPostgresStore`

**Verify:** `go build ./internal/delivery/channels/lark/... ./internal/infra/lark/...`

---

## Batch 2: Remove Auth System

**Delete directories:**
- `internal/domain/auth/` (entire)
- `internal/app/auth/` (entire)
- `internal/infra/auth/` (entire)
- `web/lib/auth/` (entire)
- `web/components/auth/` (entire)
- `web/app/login/` (entire)
- `cmd/auth-user-seed/` (entire)
- `migrations/auth/` (entire)

**Delete files:**
- `internal/delivery/server/http/auth_handler.go`
- `internal/delivery/server/http/auth_handler_test.go`
- `internal/delivery/server/http/middleware_auth.go`
- `internal/delivery/server/bootstrap/auth.go`
- `scripts/setup_local_auth_db.sh`

**Edit files:**
- `internal/delivery/server/bootstrap/server.go` — remove `BuildAuthService()` call, auth handler init
- `internal/delivery/server/bootstrap/config.go` — remove auth config loading, `AUTH_*` env vars
- `internal/delivery/server/http/router.go` — remove auth routes, auth middleware wrapping
- `internal/delivery/server/http/router_deps.go` — remove `AuthHandler`, `AuthService` fields
- `internal/delivery/server/http/middleware_rate_limit.go` — remove `CurrentUser()` usage
- `internal/delivery/server/http/middleware_context.go` — remove `authUserContextKey` if auth-only
- `internal/shared/config/file_config.go` — remove `AuthConfig` struct and field
- Web pages using `RequireAuth` / `useAuth()`:
  - `web/app/providers.tsx` — remove `AuthProvider`
  - `web/components/layout/Header.tsx` — remove auth UI
  - `web/app/conversation/page.tsx` — remove `RequireAuth` guard
  - `web/app/sessions/page.tsx`, `web/app/sessions/details/page.tsx`
  - `web/app/evaluation/page.tsx`
  - `web/app/dev/**` pages (configuration, context-config, context-window, diagnostics, etc.)
  - `web/hooks/useSSE/useSSE.ts` — remove auth token injection

**Verify:** `go build ./...` + `cd web && npm run build`

---

## Batch 3: Remove Sandbox

**Delete directories:**
- `internal/infra/sandbox/` (entire — client.go, types.go)
- `internal/infra/tools/builtin/sandbox/` (entire — 9 files)

**Delete files:**
- `internal/devops/services/sandbox.go`
- `scripts/lib/common/sandbox.sh`
- `docs/operations/SANDBOX_INTEGRATION.md`

**Edit files:**
- `internal/app/toolregistry/registry_builtins.go` — remove sandbox tool registration branch, keep only local tools (the `ToolsetLarkLocal` path becomes the only path)
- `internal/app/toolregistry/toolset.go` — simplify: remove `ToolsetDefault` vs `ToolsetLarkLocal` distinction (single toolset)
- `internal/delivery/server/bootstrap/server.go` — remove `sandbox.NewClient()` creation
- `internal/delivery/server/http/router_deps.go` — remove `SandboxClient` field
- `internal/delivery/server/http/router.go` — remove `WithSandboxClient()` option
- `internal/delivery/server/http/api_handler.go` — remove `SandboxClient` interface and field, `WithSandboxClient()` option
- `internal/delivery/server/http/api_handler_misc.go` — remove `HandleSandboxBrowserInfo()`, `HandleSandboxBrowserScreenshot()`
- `dev.sh` — remove sandbox commands, env vars, start/stop logic
- `internal/shared/config/file_config.go` — remove sandbox config fields (`SandboxBaseURL`, etc.)

**Verify:** `go build ./...`

---

## Batch 4: Remove Remaining Postgres Stores + DI Simplification

**Delete files:**
- `internal/infra/session/postgresstore/` (entire directory)
- `internal/infra/kernel/postgres_store.go` (+ test if exists)
- `internal/infra/session/state_store/postgres_store.go` (+ test if exists)
- `internal/infra/task/postgres_store.go` (+ test if exists)
- `internal/delivery/server/app/postgres_event_history_store.go`
- `internal/delivery/server/app/async_event_history_store.go`
- `internal/shared/testutil/postgres.go`
- `migrations/materials/` (entire — no Go adapter exists)

**New file — Kernel file-based dispatch store:**
- `internal/infra/kernel/file_store.go` — implements `domain/kernel.Store`
- Design:
  - Storage: `{dir}/dispatches.json` (single JSON file, <100 records typical)
  - In-memory map `map[string]Dispatch` + RWMutex
  - Atomic writes via temp file + rename (same pattern as Lark file stores)
  - `EnqueueDispatches()` — append to map, persist, return created dispatches
  - `ClaimDispatches()` — filter pending, sort by priority DESC/created_at ASC, set lease_owner+lease_until, persist
  - `MarkDispatch{Running,Done,Failed}()` — update status in map, persist
  - `RecoverStaleRunning()` — scan for running dispatches past lease_until, mark failed
  - `ListActiveDispatches()` — filter non-terminal statuses from map
  - `ListRecentByAgent()` — group by agent_id, return most recent per agent
  - Low-volume (~1 cycle per 10 min), concurrency handled by mutex

**New file — Event history file-based store:**
- `internal/delivery/server/app/file_event_history_store.go` — implements `EventHistoryStore`
- Design:
  - Storage: `{dir}/events/{session_id}.jsonl` (one JSONL file per session)
  - Append-only writes, no in-memory index needed
  - `Append()` — marshal event to JSON, append line to session file
  - `AppendBatch()` — batch-append multiple lines (for async wrapper compat)
  - `Stream()` — open session file, read line-by-line, filter by event_types, call `fn(event)` per matching line
  - `DeleteSession()` — remove session file
  - `HasSessionEvents()` — check file existence
  - Retention: optional background prune of old session files by mtime
  - Attachment handling: reuse existing `sanitizePayload()` logic (strip binary, keep small inline)
  - The `AsyncEventHistoryStore` wrapper can be reused as-is on top of this

**Edit files — DI layer:**
- `internal/app/di/builder_session.go` — remove `buildPostgresResources()`, pool creation, `SessionDatabaseURL` logic; always use file-based stores; remove `SessionDB` from return; task store → nil (server bootstrap creates InMemoryTaskStore)
- `internal/app/di/container.go` — remove `SessionDB *pgxpool.Pool` field; remove `TaskStore` field (or keep nil)
- `internal/app/di/builder_hooks.go` — change `buildKernelEngine()` to use `kernel.NewFileStore()` instead of `kernel.NewPostgresStore(pool)`

**Edit files — Server bootstrap:**
- `internal/delivery/server/bootstrap/server.go` — replace `PostgresEventHistoryStore` with `FileEventHistoryStore`; remove `container.SessionDB` usage; use `InMemoryTaskStore` (with file persistence) as the only task store
- `internal/delivery/server/bootstrap/config.go` — remove `SessionDatabaseURL`, `RequireSessionDatabase` config

**Edit files — Config:**
- `internal/shared/config/file_config.go` — remove `SessionDatabaseURL` and related fields

**Verify:** `go build ./...` + `go test ./internal/app/di/... ./internal/infra/kernel/... ./internal/delivery/server/app/...`

---

## Batch 5: Final Cleanup

- `go mod tidy` — removes `pgx`, `pgpassfile`, `pgservicefile`, `puddle`, `golang-jwt`, `argon2id` deps
- Remove `AUTH_*`, `ALEX_SESSION_DATABASE_URL`, `SANDBOX_*` from `.env.example` / `.env`
- Remove `auth:` and `sandbox_base_url` sections from `configs/config.yaml`
- Remove postgres/sandbox services from `docker-compose.yml` / `docker-compose.dev.yml`
- Full lint + test: `go vet ./...`, `golangci-lint run`, `go test ./...`
- Web build: `cd web && npm run lint && npm run build`

---

## Verification Checklist

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes (excluding tests that need real DB)
- [ ] No `pgx` or `pgxpool` imports remain (grep verify)
- [ ] No `sandbox` imports remain in non-devops code
- [ ] No `auth` imports remain
- [ ] `cd web && npm run build` passes
- [ ] `go mod tidy` produces no diff
- [ ] `alex dev restart backend` starts without DB
- [ ] Lark gateway starts and handles messages

## Impact

**Removed features:**
- User login / multi-user auth (removed entirely)
- Sandbox remote tool execution (removed entirely)
- Materials storage (removed entirely — was migration-only, no Go code)

**Preserved features (file-based):**
- Session persistence (filestore — already existed)
- State/turn snapshots (file store — already existed)
- Task tracking (InMemoryTaskStore + file persistence — already existed)
- Kernel proactive agent loop (NEW file-based dispatch store)
- Event history / web dashboard replay (NEW file-based JSONL store)
- Lark task registry (file store — already existed)
- Lark chat-session bindings (file store — already existed)
- Lark plan review state (file store — already existed)
- Lark OAuth tokens (file store — already existed)
- Cost tracking (always file-based)
- Memory engine (always file/markdown)
- Checkpoints (always file-based)
- All local tools (shell, file ops, browser, code exec)
