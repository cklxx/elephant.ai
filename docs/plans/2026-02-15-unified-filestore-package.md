# Unified `filestore` Package — Full Migration Plan

## Context

The codebase has 12+ independent file-based stores, each reinventing directory creation, tilde expansion, mutex locking, JSON marshalling, and atomic writes. This creates ~800 lines of duplicated boilerplate and inconsistent durability guarantees (some stores lack atomic writes, a latent corruption risk). This plan consolidates all file-based storage behind a single `internal/infra/filestore/` package with layered primitives and a generic `Collection[K,V]` type.

## Architecture

```
internal/infra/filestore/
├── atomic.go        — AtomicWrite, ReadFileOrEmpty, EnsureDir, EnsureParentDir, ResolvePath
├── collection.go    — Collection[K,V] generic in-memory map + file persistence
├── eviction.go      — EvictByTTL, EvictByCap composable helpers
├── atomic_test.go
├── collection_test.go
└── eviction_test.go
```

**Layer 1 (primitives)**: Stateless functions for any store to use.
**Layer 2 (Collection)**: Generic type for Pattern A stores (in-memory map + single JSON file).
**Layer 3 (eviction)**: Optional helpers plugged into `Collection.Mutate`.

## Key Design Decisions

| Decision | Rationale |
|---|---|
| `Mutate(fn)` escape hatch over rich query API | Domain stores have wildly different mutation patterns (lease-claim, bucket-eviction, status transitions). `Mutate` gives direct map access under the lock. |
| Custom `marshalDoc`/`unmarshalDoc` callbacks | Each store has a different JSON envelope (`{"dispatches":[...]}`, `{"tasks":[...]}`, flat map). Preserves on-disk format without breaking existing data. |
| Server InMemoryTaskStore gets primitives only | It manages 3 linked maps (`tasks`, `owners`, `leases`). Forcing into single `Collection` would be awkward. Primitives still eliminate ~20 lines. |
| `ResolvePath` replaces 3 tilde-expansion implementations | Single tested function eliminates subtle divergence. |

## Migration Phases

### Phase 1: Foundation — Create `internal/infra/filestore/` Package

**Files to create:**
- `internal/infra/filestore/atomic.go`
- `internal/infra/filestore/collection.go`
- `internal/infra/filestore/eviction.go`
- `internal/infra/filestore/atomic_test.go`
- `internal/infra/filestore/collection_test.go`
- `internal/infra/filestore/eviction_test.go`

**`atomic.go`** — stateless primitives:
- `EnsureDir(path string) error` — `os.MkdirAll(path, 0o755)`
- `EnsureParentDir(filePath string) error` — `EnsureDir(filepath.Dir(filePath))`
- `AtomicWrite(filePath string, data []byte, perm os.FileMode) error` — write to `.tmp` + `os.Rename`
- `ReadFileOrEmpty(path string) ([]byte, error)` — returns `(nil, nil)` for missing file
- `ResolvePath(configured, defaultPath string) string` — tilde + env expansion (extract from `container.go:372-407`)

**`collection.go`** — generic type:
- `Collection[K comparable, V any]` with `RWMutex`, in-memory `map[K]V`, optional file-backed persistence
- Methods: `Load`, `EnsureDir`, `Get`, `Put`, `Delete`, `Len`, `Snapshot`, `Mutate`, `MutateWithRollback`, `ReadLocked`
- `CollectionConfig{FilePath, Perm, Name}`
- `Now func() time.Time` exported field for test injection
- Custom marshal/unmarshal callbacks for envelope control

**`eviction.go`** — composable helpers:
- `EvictByTTL[K, V](items map[K]V, now time.Time, maxAge time.Duration, ageFn func(V) time.Time) int`
- `EvictByCap[K, V](items map[K]V, maxCap int, ageFn func(V) time.Time) int`

**Also:**
- Extract `resolveStorageDir` from `internal/app/di/container.go:372-407` → call `filestore.ResolvePath`

### Phase 2: Pilot — ChatSessionBindingLocalStore

**File:** `internal/delivery/channels/lark/chat_session_binding_local.go`

Simplest Pattern A store (3 domain methods, no TTL). Rewrite to embed `*filestore.Collection[string, ChatSessionBinding]`. Keep domain interface unchanged. Run existing tests. Add concurrency stress test.

### Phase 3: Remaining Lark Stores (increasing complexity)

1. **PlanReviewLocalStore** (`internal/delivery/channels/lark/plan_review_local.go`)
   - Uses `Mutate` + `EvictByTTL` for expired item cleanup

2. **OAuth FileTokenStore** (`internal/infra/lark/oauth/token_store_file.go`)
   - Cross-package usage validation. Uses default marshal (flat map, no envelope).

3. **TaskLocalStore** (`internal/delivery/channels/lark/task_store_local.go`)
   - Most complex: `Mutate` with TTL eviction + per-chat cap eviction. Domain-specific bucket logic stays in domain store.

### Phase 4: Kernel FileStore

**File:** `internal/infra/kernel/file_store.go`

Migrate to `Collection[string, kernel.Dispatch]`:
- `MutateWithRollback` for `EnqueueDispatches` (current rollback pattern)
- `Mutate` for `ClaimDispatches`, `MarkDispatch*`
- `ReadLocked` for `ListActiveDispatches`, `ListRecentByAgent`
- Custom marshal preserving `{"dispatches":[...]}` envelope with deterministic sort

### Phase 5: Primitive Adoption for Pattern B/C/D Stores

These stores do NOT use in-memory maps — they benefit from Layer 1 primitives only.

| Store | File | Changes |
|---|---|---|
| Session FileStore | `internal/infra/session/filestore/store.go` | `ResolvePath` for `~/`, **`AtomicWrite` in `Save`** (fixes non-atomic write bug) |
| State Store | `internal/infra/session/state_store/file_store.go` | **`AtomicWrite` in `SaveSnapshot`** (fixes non-atomic write) |
| Checkpoint Store | `internal/domain/agent/react/checkpoint.go` | **`AtomicWrite` in `Save`/`SaveArchive`** (fixes non-atomic write) |
| Cost Store | `internal/infra/storage/cost_store.go` | `ResolvePath` for `~/`, `AtomicWrite` for session index writes |
| ConfigAdmin Store | `internal/shared/config/admin/store.go` | `EnsureParentDir`, **`AtomicWrite` in `SaveOverrides`** (fixes non-atomic write) |

**Bug fixes**: 4 stores currently use non-atomic `os.WriteFile` — migrating to `AtomicWrite` fixes latent corruption risk.

### Phase 6: Server InMemoryTaskStore (Partial Adoption)

**File:** `internal/delivery/server/app/task_store.go`

Partial: replace `persistLocked()` body with `AtomicWrite()`, `loadFromDisk()` with `ReadFileOrEmpty()`, `os.MkdirAll` with `EnsureParentDir()`. Keep 3-map design intact.

### Phase 7: Cleanup

- Remove old `resolveStorageDir` if fully replaced
- Evaluate whether `internal/infra/storage/local.go` (Storage Manager) should be deprecated or simplified to use `filestore` primitives
- Update imports, run full CI

## Critical Files

| File | Role |
|---|---|
| `internal/app/di/container.go:372-407` | `resolveStorageDir` to extract |
| `internal/delivery/channels/lark/chat_session_binding_local.go` | Pilot target |
| `internal/delivery/channels/lark/plan_review_local.go` | Phase 3 migration |
| `internal/delivery/channels/lark/task_store_local.go` | Phase 3 migration (complex) |
| `internal/infra/lark/oauth/token_store_file.go` | Phase 3 migration |
| `internal/infra/kernel/file_store.go` | Phase 4 migration |
| `internal/infra/session/filestore/store.go` | Phase 5 (AtomicWrite fix) |
| `internal/infra/session/state_store/file_store.go` | Phase 5 (AtomicWrite fix) |
| `internal/domain/agent/react/checkpoint.go` | Phase 5 (AtomicWrite fix) |
| `internal/infra/storage/cost_store.go` | Phase 5 (ResolvePath + AtomicWrite) |
| `internal/shared/config/admin/store.go` | Phase 5 (AtomicWrite fix) |
| `internal/delivery/server/app/task_store.go` | Phase 6 (partial) |
| `internal/shared/json/jsonx.go` | Reused by Collection for marshal/unmarshal |

## Verification

After each phase:
1. Existing tests pass unchanged (primary correctness gate)
2. `go vet ./...` and `golangci-lint run` pass
3. Golden-file test: compare JSON output of old vs new on same input (backward compat)
4. Concurrency stress test: 100 goroutines doing Put/Get/Delete
5. After all phases: `make ci-local` / `scripts/pre-push.sh`

Code review via `skills/code-review/SKILL.md` on the diff before committing each phase.
