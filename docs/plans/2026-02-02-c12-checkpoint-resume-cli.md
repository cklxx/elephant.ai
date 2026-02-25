# Plan: C12 Checkpoint Integration Test + CLI Resume Command

**Date:** 2026-02-02
**Status:** done
**Owner:** cklxx

## Goal
- CLI supports resuming a session from a persisted ReAct checkpoint.
- Integrate checkpoint persistence into DI so runtime resume is functional.
- Add an integration test that exercises resume through the coordinator pipeline.

## Scope
- DI wiring of `react.CheckpointStore` (file-backed) and exposure for CLI.
- CLI `resume` command with validation and clear errors when checkpoint missing.
- Integration test covering resume restore + checkpoint deletion.
- Update task status in `docs/plans/2026-02-01-task-split-claude-codex.md`.

## Non-goals
- New checkpoint storage backend (DB/object storage).
- UI changes or web API additions for resume.

## Plan
1. **Checkpoint store wiring**
   - Create a `react.FileCheckpointStore` under the session dir (e.g., `${session_dir}/checkpoints`).
   - Pass store into `agentcoordinator.WithCheckpointStore`.
   - Expose the store on `di.Container` for CLI use.

2. **CLI resume command**
   - Add `alex resume <session-id>`.
   - Validate `CheckpointStore` exists and a checkpoint is present before running.
   - Run task with streaming output using the provided session ID.

3. **Integration test**
   - In coordinator package, seed a checkpoint for a session.
   - Execute with empty task and assert messages include checkpoint state.
   - Ensure checkpoint file is deleted after resume.

4. **Docs + status updates**
   - Update `docs/plans/2026-02-01-task-split-claude-codex.md` C12 status + commit.

## Acceptance Criteria
- `alex resume <session-id>` resumes when checkpoint exists, errors otherwise.
- Coordinator resume integration test passes and validates checkpoint deletion.
- All lint/tests pass; dev services restarted.

## Tests
- `go test ./internal/agent/app/coordinator -run Checkpoint` (new test)
- Full: `./dev.sh lint` and `./dev.sh test`

## Assumptions
- File-based checkpoints under session dir are acceptable for CLI usage.

## Result
- Implemented in `624869e4`.
