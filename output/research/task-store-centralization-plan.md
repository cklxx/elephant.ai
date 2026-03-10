# Task Store Centralization Plan

## Quick Win

The single most impactful quick win is:

1. build one durable `internal/domain/task.Store` in DI,
2. assign it to `container.TaskStore`,
3. switch the HTTP server and Lark gateway to consume it through the existing adapters,
4. leave Telegram as a follow-on unless we want to add one more adapter in the same change.

This is the highest-leverage move because the server and Lark paths are the main places where long-running work is created, resumed, tracked, and surfaced to users, and both already have adapter shims ready.

## Current Bypass Paths

### 1. DI declares a unified task store but never builds it

- `internal/app/di/container.go`
  - `Container` already has `TaskStore taskdomain.Store`.
- `internal/app/di/container_builder.go`
  - `container.TaskStore` is never initialized.

Impact:

- the intended central source of truth exists in the type system only,
- every runtime path below falls back to delivery-local stores.

### 2. HTTP server creates its own local in-memory/file-persisted store

- `internal/delivery/server/bootstrap/server.go`
  - constructs `serverApp.NewInMemoryTaskStore(...)`
  - persists to `filepath.Join(container.SessionDir(), "_server", "tasks.json")`
  - passes that delivery-local store into:
    - `serverApp.NewTaskProgressTracker(...)`
    - `serverApp.NewTaskExecutionService(...)`

Supporting local-store implementation files:

- `internal/delivery/server/app/task_store.go`
- `internal/delivery/server/app/task_store_crud.go`
- `internal/delivery/server/app/task_store_ops.go`

Impact:

- web/server tasks are isolated from Lark/runtime ownership,
- resume/lease state is stored in a server-only format,
- the existing `internal/delivery/taskadapters/server_adapter.go` is effectively unused in production wiring.

### 3. Lark gateway creates its own channel-local task store

- `internal/delivery/server/bootstrap/lark_gateway.go`
  - calls `buildLarkTaskStore(...)`
  - chooses `lark.NewTaskMemoryStore(...)` or `lark.NewTaskFileStore(...)`
  - default file mode writes under `filepath.Join(container.SessionDir(), "lark")`

Supporting local-store implementation files:

- `internal/delivery/channels/lark/task_store.go`
- `internal/delivery/channels/lark/task_store_local.go`

Unused adapter that already solves the interface mismatch:

- `internal/delivery/taskadapters/lark_adapter.go`

Impact:

- Lark background tasks are persisted separately from server tasks,
- ownership is chat-scoped rather than product-scoped,
- restarts can recover local Lark state, but not through the unified domain model.

### 4. Telegram has the same smell, but lower leverage

- `internal/delivery/server/bootstrap/telegram_gateway.go`
  - calls `buildTelegramTaskStore(...)`
  - chooses `telegram.NewTaskMemoryStore(...)` or `telegram.NewTaskFileStore(...)`

Supporting local-store implementation files:

- `internal/delivery/channels/telegram/task_store.go`
- `internal/delivery/channels/telegram/task_store_local.go`

Impact:

- same fragmentation pattern as Lark,
- but there is no existing Telegram adapter equivalent to `taskadapters/lark_adapter.go`, so it is not the fastest first cut.

## Target End State

### Desired ownership model

Use one shared task ledger for all leader-owned work:

- one durable `internal/domain/task.Store`
- built once in DI
- injected into the server via `taskadapters.ServerAdapter`
- injected into Lark via `taskadapters.LarkAdapter`
- optionally later injected into Telegram via a new `taskadapters.TelegramAdapter`

This keeps existing delivery-layer interfaces stable while centralizing persistence.

## Concrete Fix Plan

## Step 1: Add a real domain task store implementation and build it in DI

### Files to add

- `internal/infra/taskstore/local_store.go`
- `internal/infra/taskstore/local_store_crud.go`
- `internal/infra/taskstore/local_store_query.go`
- `internal/infra/taskstore/local_store_lease.go`
- `internal/infra/taskstore/local_store_test.go`

### Proposed implementation

- Create a durable local adapter that implements `internal/domain/task.Store`.
- Reuse the proven behavior from the current server store:
  - file persistence,
  - lease claiming,
  - resume queries,
  - retention/eviction,
  - transition updates.
- Back it with a shared file under a neutral path such as:
  - `filepath.Join(container.SessionDir(), "_tasks", "tasks.json")`

Why this shape:

- it is the smallest implementation step,
- it avoids introducing a DB migration just for the quick win,
- it preserves current single-node assumptions already present elsewhere in the repo.

### Files to change

- `internal/app/di/container_builder.go`
- `internal/app/di/container.go`

### Proposed changes

- Add a `buildTaskStore()` helper in DI.
- Instantiate the shared store during `Build()`.
- Assign it to `container.TaskStore`.
- If the shared store supports close/drain, register it in `container.Drainables`.

## Step 2: Switch HTTP server bootstrap to the shared store through the existing adapter

### Files to change

- `internal/delivery/server/bootstrap/server.go`
- `internal/delivery/taskadapters/server_adapter.go` if any small adapter gap appears

### Proposed changes

- Replace:
  - `serverApp.NewInMemoryTaskStore(...)`
- With:
  - `taskadapters.NewServerAdapter(container.TaskStore)`

- Continue using:
  - `serverApp.NewTaskProgressTracker(taskStore)`
  - `serverApp.NewTaskExecutionService(..., taskStore, ...)`

This is low risk because those services already depend on `internal/delivery/server/ports.TaskStore`, and `ServerAdapter` already implements that interface.

### Important cleanup

- Remove direct ownership of the delivery-local store from bootstrap.
- Keep the delivery-local store package temporarily only if tests still depend on it.

## Step 3: Switch Lark bootstrap to the shared store through the existing adapter

### Files to change

- `internal/delivery/server/bootstrap/lark_gateway.go`
- `internal/delivery/taskadapters/lark_adapter.go` only if minor mapping gaps appear

### Proposed changes

- In `startLarkGateway(...)`, prefer:
  - `taskadapters.NewLarkAdapter(container.TaskStore)`
- Stop calling `buildLarkTaskStore(...)` when `container.TaskStore != nil`.
- Retain `buildLarkTaskStore(...)` only as an explicit fallback if we want a staged rollout.

Recommended quick-win behavior:

- if shared store initialization succeeded, always use it,
- if shared store initialization failed, fail startup or log a degraded-mode warning rather than silently fragmenting ownership again.

### Why this is high impact

- background task completion, polling recovery, and `/task` views in Lark all immediately converge on the same ledger the server uses.

## Step 4: Leave Telegram as a phase-2 follow-on

### Files to change later

- `internal/delivery/server/bootstrap/telegram_gateway.go`
- add `internal/delivery/taskadapters/telegram_adapter.go`

### Proposed change later

- mirror the Lark approach after the shared store is stable.

Rationale:

- Telegram is not the fastest route to the leader-agent ownership story,
- there is no ready-made adapter today,
- server + Lark captures the highest-value user-facing workflows first.

## Step 5: Handle legacy on-disk data migration

### Legacy sources to consider

- server tasks:
  - `filepath.Join(container.SessionDir(), "_server", "tasks.json")`
- Lark tasks:
  - default `filepath.Join(container.SessionDir(), "lark", "tasks.json")`
- Telegram tasks:
  - default `filepath.Join(container.SessionDir(), "telegram", "telegram_tasks.json")`

### Proposed migration behavior

- Add a one-time import in the new shared store builder:
  - load shared file if present,
  - if empty, best-effort import from legacy server and Lark files,
  - normalize into `internal/domain/task.Task` records,
  - write the shared file,
  - leave legacy files untouched for one release.

Why:

- this preserves current in-flight ownership data,
- avoids abrupt task loss after the wiring switch.

## Files Most Likely To Change In The Quick Win

- `internal/app/di/container.go`
- `internal/app/di/container_builder.go`
- `internal/delivery/server/bootstrap/server.go`
- `internal/delivery/server/bootstrap/lark_gateway.go`
- `internal/delivery/taskadapters/server_adapter.go`
- `internal/delivery/taskadapters/lark_adapter.go`
- `internal/infra/taskstore/local_store.go`
- `internal/infra/taskstore/local_store_crud.go`
- `internal/infra/taskstore/local_store_query.go`
- `internal/infra/taskstore/local_store_lease.go`

## Tests To Add Or Update

### New tests

- `internal/infra/taskstore/local_store_test.go`
  - create/get/list/set-status/update-progress
  - claim/renew/release lease
  - claim resumable tasks
  - bridge meta persistence
  - file reload
  - legacy import from old server/Lark files

- `internal/delivery/server/bootstrap/server_task_store_wiring_test.go`
  - server bootstrap uses `container.TaskStore` via `taskadapters.ServerAdapter`
  - no `NewInMemoryTaskStore()` path in production wiring when shared store is available

- `internal/delivery/server/bootstrap/lark_task_store_wiring_test.go`
  - Lark bootstrap uses `container.TaskStore` via `taskadapters.LarkAdapter`
  - stale-running cleanup still runs through the shared store adapter

### Existing tests to keep

- `internal/delivery/taskadapters/adapters_test.go`
  - already validates most adapter-level mapping
- existing `internal/delivery/server/app/task_store_test.go`
  - keep temporarily if the old delivery-local store remains for tests or phased removal

## Suggested Execution Order

1. Add the new infra store implementing `internal/domain/task.Store`.
2. Wire it into DI and populate `container.TaskStore`.
3. Switch server bootstrap to `taskadapters.ServerAdapter`.
4. Switch Lark bootstrap to `taskadapters.LarkAdapter`.
5. Add import/migration from legacy store files.
6. Add wiring tests.
7. Decide whether to remove or deprecate the old delivery-local task stores.

## Recommendation

For the quick win, do not try to centralize every task surface at once.

Ship this slice first:

- shared domain task store in DI,
- server bootstrap uses it,
- Lark bootstrap uses it,
- legacy server/Lark file import,
- Telegram explicitly deferred.

That delivers the core product outcome with the least architectural churn: the leader agent finally has one ownership ledger for the two highest-value execution surfaces.
