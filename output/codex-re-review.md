# Fix Re-Review

Date: 2026-03-10
Scope: re-review of fix commits `f12917d2`, `fac396ed`, `4bf78bbc`, `9203b2dd`, `c75123c3`.

## Verdicts

### `f12917d2` `fix(bootstrap): wire JSON Schema validation into startup Phase 1`

Verdict: **PASS**

- The original P1 finding is fixed: `BootstrapFoundation` now calls `validateConfigSchema(logger)` immediately after config load and before container build in `internal/delivery/server/bootstrap/foundation.go:52-79`.
- The helper resolves the config path, reads the YAML, and logs each warning returned by `ValidateConfigSchema` without blocking startup in `internal/delivery/server/bootstrap/foundation.go:239-255`.
- `go test ./internal/delivery/server/bootstrap` passed.

Remaining issue:

- The earlier P2 limitation still exists: `internal/shared/config/validate_schema.go` is still a hand-rolled subset validator rather than a full JSON Schema implementation. This commit correctly wires validation, but it does not broaden schema coverage.

### `fac396ed` `fix(lark): close TOCTOU race in event dedup first-delivery check`

Verdict: **FAIL**

- The commit fixes the absent-key race by replacing `Load` + `Store` with `LoadOrStore`; the new first-delivery test is good for that exact case.
- But the fix is incomplete for expired entries. In `internal/delivery/channels/lark/dedup.go:70-87`, if the key already exists with an expired timestamp, multiple goroutines can all observe `loaded=true`, all conclude the entry is expired, all `Store` a fresh expiry, and all continue as non-duplicates. That still allows duplicate processing during the stale-entry window before sweep removes the old key.
- The old cleanup-test gap also remains: `TestEventDedup_CleanupGoroutine` still does not shorten the 1-minute ticker in `internal/delivery/channels/lark/dedup.go:13,40`, so the test in `internal/delivery/channels/lark/dedup_test.go:156+` does not actually prove the background sweep ran.
- `go test ./internal/delivery/channels/lark` passed, but it does not cover the expired-entry race.

### `4bf78bbc` `fix(health): sanitize per-model health data exposed via /health endpoint`

Verdict: **FAIL**

- The commit does remove the raw-error leak: `LLMModelHealthProbe` now sanitizes `[]llm.ProviderHealth` to `[]llm.SanitizedHealth` in `internal/delivery/server/app/health.go:93-130`, and the new tests verify raw provider names, endpoints, and secret-like strings are no longer serialized.
- However, the public `/health` endpoint still exposes model-level telemetry to unauthenticated callers. The route remains public in `internal/delivery/server/http/router.go:168`, the server still registers `LLMModelHealthProbe` in `internal/delivery/server/bootstrap/server.go:182-184`, and the sanitized payload still includes `model`, `error_rate`, `health_score`, and `last_checked` in `internal/infra/llm/health.go:39-45`.
- That means the original public information-disclosure problem is reduced but not fully closed: model enumeration and live health telemetry are still disclosed publicly.
- `go test ./internal/delivery/server/app ./internal/infra/llm ./internal/delivery/server/bootstrap` passed.

### `9203b2dd` `fix(memory): stop cleanup goroutine on container shutdown`

Verdict: **FAIL**

- The normal shutdown path is fixed: `Build()` now creates `bgCtx, bgCancel`, stores `bgCancel` on the container, and `Shutdown()` cancels it in `internal/app/di/container_builder.go:85-90`, `internal/app/di/container.go:156-158`.
- The memory cleanup loop honors context cancellation and now logs on exit in `internal/infra/memory/cleanup.go:97-120`. The new cleanup-loop tests also pass.
- But the fix is still incomplete on build failures after memory initialization. After `buildMemoryEngine(bgCtx)` succeeds, several later steps can still return early without calling `bgCancel`, including `buildCostTracker`, `NewSLACollector`, `buildToolRegistry`, and `teamrun.NewFileRecorder` in `internal/app/di/container_builder.go:101-127`.
- If memory cleanup has already started, those error returns still leak the background goroutine because no container is returned to call `Shutdown()`.
- `go test ./internal/infra/memory` passed. `go test ./internal/app/di` did not pass cleanly because of an unrelated existing failure: `TestExternalAgentDispatch_HasSingleRuntimeExecuteEntryPoint`.

### `c75123c3` `fix: block file tool path traversal`

Verdict: **PASS**

- The original workspace-escape issue is fixed centrally in `internal/infra/tools/builtin/pathutil/path_guard.go:13-58`: `ResolveLocalPath` and `SanitizePathWithinBase` now reject paths that escape the workspace root, while `ResolveLocalPathOrTemp` preserves the temp-file exception needed by attachment flows.
- The guard uses `pathWithinBase` after normalization and symlink resolution, so `..`, absolute outside-workspace paths, and symlink escapes are all blocked.
- Regression coverage is good:
  - `internal/infra/tools/builtin/pathutil/path_guard_test.go` now rejects traversal, outside absolute paths, and symlink escapes.
  - `internal/infra/tools/builtin/aliases/file_path_validation_test.go` verifies `read_file`, `write_file`, and `replace_in_file` reject `../../etc/passwd`.
- `go test ./internal/infra/tools/builtin/pathutil ./internal/infra/tools/builtin/aliases` passed.

## Summary

- Pass: `f12917d2`, `c75123c3`
- Fail: `fac396ed`, `4bf78bbc`, `9203b2dd`

Highest-priority remaining fixes:

1. Make dedup refresh atomic for expired entries, not just absent ones.
2. Remove or gate `llm_models` telemetry from the public `/health` route.
3. Ensure `bgCancel()` runs on all post-memory-init `BuildContainer` error paths.
