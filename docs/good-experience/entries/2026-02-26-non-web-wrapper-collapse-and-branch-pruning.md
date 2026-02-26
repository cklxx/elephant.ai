# 2026-02-26 — Non-web wrapper collapse and branch-pruning pass

Impact: Reduced cognitive overhead in CLI/config/subscription flows by removing single-use indirection and duplicated defensive branches while keeping behavior stable.

## What changed

- Removed single-use wrappers in CLI command paths (`config`, `setup`, `model`) so command dispatch goes directly to core `*With` implementations.
- Consolidated duplicated MCP add/remove save flow into shared helpers (`loadMCPConfig`, `saveMCPConfig`).
- Unified repetitive export option value parsing in `handleCostExport` with `requireOptionValue`.
- Pruned duplicate/defensive branches that were already covered by centralized logic:
  - selection store clear path now delegates final empty-file handling to `saveDocLocked`
  - onboarding state store reuses shared `contextErr` checks
  - proactive level normalization uses one defaulting branch
  - removed pre-loop empty checks in safe map/slice normalization paths
- Removed duplicate persona voice fallback logic via one shared helper.

## Why this worked

- Simplifications were applied only where behavior was already guaranteed by existing contracts or centralized helpers.
- Shared helper extraction was used only when call sites already had obvious duplicated structure.
- Existing tests around command behavior and persistence were reused to validate behavior preservation.

## Validation

- `./scripts/go-with-toolchain.sh test ./cmd/alex ./internal/app/subscription ./internal/app/context ./internal/infra/session/filestore`
- `./scripts/go-with-toolchain.sh test ./internal/shared/config -run 'TestLoadNormalizesRuntimeConfig|TestLoadFromFile|TestLoadDefaults|TestMergeProactiveConfig_IncludesPromptAndTimerHeartbeat'`
- `python3 skills/code-review/run.py '{"action":"review"}'` + manual P0/P1 review

## Metadata
- id: good-2026-02-26-non-web-wrapper-collapse-branch-pruning
- tags: [good, maintainability, readability, simplification, cli, config]
- links:
  - docs/plans/2026-02-26-non-web-systematic-maintainability-optimization.md
  - cmd/alex/cli_model.go
  - cmd/alex/mcp.go
  - cmd/alex/cost.go
  - internal/app/subscription/selection_store.go
  - internal/shared/config/load.go
