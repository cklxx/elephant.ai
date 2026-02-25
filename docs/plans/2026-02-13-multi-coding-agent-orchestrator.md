# 2026-02-13 Multi Coding Agent Orchestrator

## Context
- Goal: implement a multi coding-agent orchestration layer on top of existing external agent support.
- Scope: coding orchestration core only (Codex + Claude Code + Lark full automation), no Happy cloud/mobile stack replication.
- Autonomy default: full autonomous execution.

## Decisions Locked
- Keep existing ReAct loop and BackgroundTaskManager; extend instead of replacing.
- Add normalized execution controls (`execution_mode`, `autonomy_level`) and map to external agent specific configs.
- Keep coding tasks isolated by default (`workspace_mode=worktree`) with verify/retry/auto-merge defaults.

## Implementation Checklist
- [x] Create dedicated worktree and branch from `main`; copy `.env`.
- [x] Extend public types/configuration for orchestration mode.
- [x] Update `bg_dispatch` tool to accept execution controls.
- [x] Add coding orchestrator DAG planner/executor over background dispatch.
- [x] Extend bridge metadata and execution mapping for codex/claude plan/execute modes.
- [x] Integrate orchestrator path into Lark task commands.
- [x] Add/adjust unit tests and integration coverage.
- [x] Add real `/codex` + `/cc` Lark message-level integration tests through the project gateway entry.
- [x] Add architecture guard tests to lock external coding-agent unique wiring and runtime entrypoint.
- [x] Run full lint + tests.
- [x] Run mandatory code review flow and capture findings.
- [x] Commit in incremental slices.
- [x] Merge back to `main` and remove temporary worktree.

## Progress Log
- 2026-02-13: Created implementation branch/worktree and initialized execution plan.
- 2026-02-13: Added execution control fields across dispatch/background/external request ports.
- 2026-02-13: Implemented cross-agent execution control mapping (`execute|plan`, `controlled|semi|full`) in managed executor.
- 2026-02-13: Added `bg_plan` (DAG planning + optional dispatch) and `bg_graph` tools; registered in tool registry.
- 2026-02-13: Extended codex/claude bridge flow for plan-mode behavior and metadata enrichment.
- 2026-02-13: Updated Lark direct dispatch prompt to force coding-task full-autonomy defaults.
- 2026-02-13: Extended unified task schema with execution/plan/retry fields and wired Postgres store columns.
- 2026-02-13: Ran mandatory code review, fixed two issues:
  - normalized `agent_type` to canonical values (`codex` / `claude_code`) to avoid runtime executor mismatch;
  - ensured codex plan mode honors read-only fallback from normalized request controls.
- 2026-02-13: Re-ran full validation (`make dev-lint`, `make test`) successfully.
- 2026-02-13: Split solution into 3 incremental commits, rebased to latest `main`, fast-forward merged, and removed temporary worktree/branch.
- 2026-02-13: Added real codex bridge integration test (`TestExecutor_Integration_Codex`) and managed multi-agent integration test (`TestManagedExternalExecutor_Integration_BothAgents`).
- 2026-02-13: Added Lark `/codex` + `/cc` real integration test (`TestGatewayIntegration_TaskCommandsDispatchRealExternalAgents`) to verify gateway entry -> managed executor -> bridge chain.
- 2026-02-13: Added static architecture guards to enforce unique managed-executor wiring and single `externalExecutor.Execute` runtime callsite.
