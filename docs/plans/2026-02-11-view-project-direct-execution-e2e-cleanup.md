# 2026-02-11 Plan — View-Project Direct Execution + E2E Re-evaluation Cleanup

## Goal
- Ensure explicit low-risk inspection requests (e.g., "查看本项目") execute immediately instead of requesting confirmation.
- Validate behavior with end-to-end evaluation evidence.
- Remove prior unrealistic 100%-pass E2E evaluation artifacts from tracked evaluation outputs.

## Scope
- Prompt/routing policy updates for delegated low-risk inspection intents.
- Evaluation dataset/report cleanup for stale 100%-pass results.
- Fresh E2E run and report capture.

## Steps
1. Identify autonomy/confirmation guardrails and current routing behavior for inspection intents.
2. Implement prompt/routing changes with tests (TDD where logic touched).
3. Locate and remove historical 100%-pass evaluation artifacts.
4. Run lint/tests and end-to-end evaluation; capture pass rates and failures.
5. Run mandatory code review workflow; fix findings if any.
6. Commit incrementally and merge branch back to `main`.

## Progress Log
- 2026-02-11 22:52: Initialized plan and worktree branch `fix-view-project-autonomy-e2e-20260211` from `main`; copied `.env`.
- 2026-02-11 23:00: Updated prompt/routing guardrails to execute explicit low-risk read-only inspection asks directly (no reconfirmation) while keeping approval/consent gates on `request_user`.
- 2026-02-11 23:01: Added regression coverage in `internal/app/context`, `internal/app/agent/preparation`, `internal/shared/agent/presets`, and `evaluation/agent_eval` for read-only inspect/list/check intents.
- 2026-02-11 23:01: Ran targeted tests: `go test ./internal/app/context ./internal/shared/agent/presets ./internal/app/agent/preparation ./evaluation/agent_eval` (all passed).
- 2026-02-11 23:01: Ran e2e suite: `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite_e2e_systematic.yaml --output tmp/foundation-suite-e2e-systematic-20260211-230148 --format markdown`.
- 2026-02-11 23:01: Captured e2e metrics: collections passed `27/28`, pass@1 `179/202` (`85.5%`), pass@5 `200/202` (`96.0%`), failed `3`, deliverable good `0/20`.
- 2026-02-11 23:02: Removed stale 100%-pass evaluation artifacts (`docs/analysis/2026-02-08-foundation-offline-eval-report.md`, `docs/research/2026-02-10-product-opt-hard-eval-r18-report.md`, `evaluation/swe_bench/EVALUATION_RESULTS.md`) and updated `evaluation/agent_eval/README.md` with current non-100% e2e metrics.
