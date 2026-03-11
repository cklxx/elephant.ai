# Domain Export Prune

## Scope

- Audit exported Go types, interfaces, and constants under `internal/domain/`.
- Confirm package-external production references before deletion or visibility reduction.
- Remove dead code and simplify interfaces that only preserve needless indirection.
- Validate, review, commit, and fast-forward merge back to `main`.

## Plan

1. Enumerate exported symbols in `internal/domain/**`.
2. Classify each symbol by production usage outside its defining package.
3. Remove dead exports and collapse over-abstracted interfaces with no real polymorphic callers.
4. Run focused and full validation, review, commit, and merge.

## Findings

- `internal/domain/agent/ports/agent` exposed several fragment interfaces with no package-external production callers:
  - `IDContextGetter`, `IDContextSetter`
  - `TokenEstimator`, `ContextCompressor`, `ContextWindowBuilder`, `ContextTurnRecorder`
- `internal/domain/agent/ports/storage` exposed `CostRecorder`, `CostQuerier`, and `CostExporter` without meaningful standalone usage; only the aggregate tracker interface was needed.
- Dead exported symbols under `internal/domain/agent/ports/agent`:
  - `TaskDependency`
  - `StreamCallback`
  - `StreamEvent`
  - `InputRequestClarification`
- `AgentTypeInternal` was exported even though it was only consumed within the defining package.
- `internal/domain/materialregistry` exposed migration API surface with no package-external production use:
  - `AttachmentStorer`
  - `api` package request metadata types/constants
  - several unused exported fields on `MigrationRequest`

## Changes

- Collapsed interface fragments into `IDContextReader`, `ContextManager`, and `CostTracker`.
- Removed dead exported agent types/constants that had no production callers.
- Reduced `AgentTypeInternal` visibility to package-private.
- Reduced `AttachmentStorer` visibility to package-private.
- Simplified `materialregistry` migration request shape to only the attachment payload still read by callers.
- Deleted dead `internal/domain/materialregistry/api` package and adjusted the remaining caller in `session_manager`.

## Validation

- `go test ./internal/domain/...`
- `go test ./internal/app/agent/... ./internal/app/context ./internal/app/di ./internal/infra/...`
- `go test ./...`
- `python3 skills/code-review/run.py review`
