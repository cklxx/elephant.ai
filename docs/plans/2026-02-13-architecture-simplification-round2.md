# Architecture Simplification — Round 2

## Context

Round 1 completed: Gateway split, TaskState slim, port interface removal (2), event unification, ServerCoordinator elimination. Net -336 LOC across 85 files, 106 test packages all green.

Round 2 targets the **next layer of god objects and unnecessary abstractions** identified by quantitative analysis:
- AgentCoordinator at 1,380 LOC with 46 methods (new god object)
- 7 single-impl port interfaces adding pointless indirection
- workflow_event_translator still at 961 LOC mixing SLA with translation
- shared/agent/* packages misplaced (domain-specific code in shared/)
- DI container at 889 LOC doing too much wiring

## Tracks

### Track F: Remove single-impl port interfaces
**Priority:** P0 (lowest risk, immediate cleanup)
**Estimated:** -200 LOC, remove 5-7 interface abstractions

Remove these interfaces from `internal/domain/agent/ports/`:
- `Logger` → inject `*slog.Logger` or `logging.Logger` directly
- `Clock` → inject `func() time.Time` or use `time.Now` directly
- `IDGenerator` → inject concrete type directly
- `IDContextReader` → inject concrete type directly
- `FunctionCallParser` → inject concrete type directly
- `CostStore` → inject concrete type directly (only postgres impl)

**Process:**
1. For each interface: grep all usages, count implementations
2. Replace interface field with concrete type in consumers
3. Delete the interface definition
4. Run tests

**Key files:**
- `internal/domain/agent/ports/agent/runtime_services.go` (Logger, Clock, IDGenerator, IDContextReader)
- `internal/domain/agent/ports/tools/parser.go` (FunctionCallParser)
- `internal/domain/agent/ports/storage/cost.go` (CostStore)
- `internal/domain/agent/react/engine.go` (consumer of Logger, Clock, IDGenerator)

### Track G: Split AgentCoordinator
**Priority:** P0 (1,380 LOC god object)
**Estimated:** 1,380 LOC → 4 files × ~350 LOC

Split `internal/app/agent/coordinator/coordinator.go` into:
- `coordinator.go` (~350) — ExecuteTask, PrepareExecution + constructor
- `session_manager.go` (~300) — GetSession, SaveSessionAfterExecution, ListSessions, session locking
- `registry_accessor.go` (~250) — GetToolRegistry*, GetParser, GetLLMClient, GetContextManager
- `config_resolver.go` (~250) — effectiveConfig, GetConfig, GetSystemPrompt, runtime config resolution

**Key files:**
- `internal/app/agent/coordinator/coordinator.go` (target)
- `internal/app/agent/coordinator/options.go` (option functions may need redistribution)

### Track H: Extract SLA from workflow_event_translator
**Priority:** P1 (961 LOC, mixed concerns)
**Estimated:** -150 LOC from translator, new 100 LOC SLA decorator

Move SLA collection logic out of `workflow_event_translator.go` into a separate `sla_event_decorator.go` that wraps EventListener. The translator should only translate events, not collect metrics.

**Key files:**
- `internal/app/agent/coordinator/workflow_event_translator.go`
- New: `internal/app/agent/coordinator/sla_event_decorator.go`

### Track I: Relocate shared/agent/* to domain
**Priority:** P1 (layer hygiene)
**Estimated:** ~0 LOC change, 3 package moves

Move domain-specific packages out of shared/:
- `shared/agent/presets/` → `internal/domain/agent/presets/` (agent prompt presets)
- `shared/agent/textutil/` → `internal/domain/agent/textutil/` (agent text utilities)
- `shared/signals/` → `internal/delivery/lifecycle/` or `internal/infra/lifecycle/` (shutdown signals)

**Process:** Move directories, update all import paths, verify build.

### Track J: Refactor DI container builder
**Priority:** P2 (889 LOC, readability)
**Estimated:** 889 LOC → 4 focused builders × ~200 LOC

Split `internal/app/di/container_builder.go` into:
- `container_builder.go` (~200) — top-level Build, Start, Shutdown
- `builder_llm.go` (~150) — LLM factory, credential refresher
- `builder_session.go` (~200) — session DB, stores, state, history
- `builder_tools.go` (~200) — tool registry, SLA, external executors, MCP
- `builder_hooks.go` (~100) — memory hooks, OKR, kernel alignment

## Execution Strategy

5 tracks in parallel worktrees, same pattern as Round 1:
- `opt-f-remove-single-impl` — Track F
- `opt-g-split-coordinator` — Track G
- `opt-h-extract-sla` — Track H
- `opt-i-relocate-shared` — Track I
- `opt-j-refactor-di` — Track J

Merge order: F → I → G → H → J (least conflict → most conflict)

## Verification

Each track:
1. `go build ./...`
2. `go vet ./internal/...`
3. `go test ./... -count=1 -timeout 600s`

After full merge:
- All 106+ test packages pass
- `go build ./cmd/...` succeeds
- Architecture check: `make check-arch`
