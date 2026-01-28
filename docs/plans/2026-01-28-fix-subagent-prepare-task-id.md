# Plan: Fix subagent prepare task_id mismatch (2026-01-28)

## Goal
- Ensure workflow prepare events emit the ensured task_id even when OutputContext inherited a parent task_id.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Update ExecuteTask to set `outCtx.TaskID = ensuredTaskID` before `wf.start(stagePrepare)`.
2. Add a coordinator test that reproduces mismatched OutputContext task_id and asserts prepare event uses ensured task_id.
3. Run full lint + tests.
4. Commit changes.

## Progress
- 2026-01-28: Plan created; engineering practices reviewed.
- 2026-01-28: Updated ExecuteTask to set OutputContext task_id before prepare; added coordinator test for prepare envelope task_id.
- 2026-01-28: Ran `go test ./internal/agent/app/coordinator -run TestExecuteTaskUsesEnsuredTaskIDForPrepareEnvelope`.
- 2026-01-28: Ran `./dev.sh lint`.
- 2026-01-28: Ran `./dev.sh test` (failed: web_fetch_test missing request log file; seedream_test missing streaming log; plus linker LC_DYSYMTAB warnings).
