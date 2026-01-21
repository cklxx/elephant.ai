# ACP Executor Sandbox Alignment Plan

**Goal:** Ensure ACP executor runs safely in the sandbox context with sensible defaults, prevent nested executor usage in subagents, and improve subagent UI aggregation.

## Scope
- Defaults: ACP executor address/cwd/mode/auto-approve should work without manual config.
- Safety: keep execution inside sandbox workspace (`/workspace`).
- Tooling: `acp_executor` must be unavailable to subagents.
- UI: subagent streams should be visually aggregated (avoid one card per tool event).

## Plan
1) **Backend defaults & permissions**
   - Set ACP executor defaults for addr/cwd/mode/auto-approve.
   - Ensure cwd defaults to `/workspace` (sandbox path).
   - Update config reference docs.

2) **Tool registry filtering**
   - Exclude `acp_executor` from subagent tool registry.
   - Update tests/mocks for `WithoutSubagent`.

3) **UI aggregation**
   - Render nested subagent tool events in compact line format (not full ToolOutputCard).

4) **Documentation & error experience**
   - Record the “operation rejected due to host path” incident in error experience.
   - Update AGENTS.md instruction for plan/progress logging.

## Progress Log
- 2026-01-21: Implemented plan items 1–3 (defaults, registry filter, UI aggregation) and updated AGENTS.md guidance.
- 2026-01-21: Added error-experience entry/summary for host-path rejection.
- 2026-01-21: Tests run: `./scripts/go-with-toolchain.sh test ./...`, `npm --prefix web run lint`, `npm --prefix web test`.
- 2026-01-21: Align ACP executor cwd default with runtime (fallback to current working dir when `/workspace` missing) and ensure executor events carry stable parent_task_id for UI aggregation.
- 2026-01-21: Restored `/workspace` default CWD with ACP server skipping missing cwd, added `sandbox` tool preset and ACP mode negotiation fallback.
- 2026-01-21: ACP server now ignores cwd switch failures to avoid prompt errors.
- 2026-01-21: Updated `dev.sh` to run ACP daemon inside the sandbox container by default, build a Linux `alex` binary for the container when needed, and reuse ACP ports via `.pids/acp.port`/container port mapping to keep ports stable.
- 2026-01-21: Tests run: `./dev.sh test` (Go + web) passed; `./dev.sh lint` failed due to existing lint issues in `internal/acp/client.go`, `internal/acp/errors.go`, and `internal/server/app/event_broadcaster.go`.
- 2026-01-21: ACP SSE smoke test: `python3 scripts/acp_test.py --addr http://127.0.0.1:$(cat .pids/acp.port) --cwd /workspace --prompt "ping"` succeeded.
- 2026-01-21: ACP executor now auto-generates a fallback artifact manifest when missing (flags `artifact_manifest_missing`), plus manual smoke test with the provided web page instruction succeeded (no missing-manifest error).
- 2026-01-21: Tests run: `./dev.sh test` passed; `./dev.sh lint` still fails on existing lint issues in `internal/acp/client.go`, `internal/acp/errors.go`, and `internal/server/app/event_broadcaster.go`.
- 2026-01-21: Fixed existing Go lint issues in `internal/acp/client.go`, `internal/acp/errors.go`, and `internal/server/app/event_broadcaster.go`.
- 2026-01-21: ACP executor now emits a summary event + fallback artifact manifest event so subagent streams show output even when executor returns only text.

## Notes / Risks
- If ACP executor still targets host cwd, the CLI agent may reject file ops (seen with `/Users/...` paths).
- Sandbox executor assumptions depend on ACP client running inside sandbox with `/workspace`.
