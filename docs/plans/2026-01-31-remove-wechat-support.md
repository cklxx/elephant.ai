# Plan: Remove legacy channel support

**Status**: Completed
**Date**: 2026-01-31

## Goals
- Remove legacy channel support (config + code paths) from backend/server.
- Remove legacy OAuth/login UI and provider wiring from web.
- Clean configuration/docs/tests/dependencies referencing the legacy channel.
- Remove legacy channel entries from local runtime config (keep file): `/Users/bytedance/.alex/config.yaml`.

## Plan
1. Inventory legacy channel references (backend, web, config, docs) and decide removals/updates.
2. Remove backend legacy channel code paths and config structs; update tests.
3. Remove web legacy OAuth flow and translations; update tests/types.
4. Clean docs/config examples and dependencies (go.mod/go.sum, env examples).
5. Run full lint + tests; fix or record issues.
6. Commit in multiple incremental commits.

## Progress Log
- 2026-01-31: Plan created; legacy channel references enumerated; execution started.
- 2026-01-31: Removed legacy channel code, config structs, auth endpoints, and web OAuth UI. Updated README/config docs and cleaned deps.
- 2026-01-31: `./dev.sh lint` passed; `./dev.sh test` failed with a data race in coordinator tests (`TestExecuteTaskPropagatesSessionIDToWorkflowEnvelope`, `TestExecuteTaskUsesEnsuredTaskIDForPrepareEnvelope`) plus macOS LC_DYSYMTAB linker warnings.
- 2026-01-31: Cleaned historical docs to remove legacy channel references.
- 2026-01-31: Re-ran `./dev.sh lint` + `./dev.sh test` (LC_DYSYMTAB linker warnings observed; tests passed).
