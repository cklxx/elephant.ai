# Plan: P0 Platform + Steward Implementation (2026-02-08)

## Goal
- Implement the confirmed P0 slice from the reprioritized roadmap:
  - Steward enforcement closure (activation, evidence refs, overflow compression).
  - Coding gateway MVP contract completion with verification output.
  - Evaluation automation closure with normalized report artifact surfacing.
  - Approval UX enrichment for L3/L4 safety context.

## Scope
- `internal/app/agent/{config,coordinator,preparation}/`
- `internal/domain/agent/{ports/agent,react}/`
- `internal/infra/coding/`
- `internal/infra/devops/shadow/`
- `evaluation/{agent_eval,gate}/`
- `internal/delivery/server/{app,http}/`
- `cmd/alex/`
- Lint baseline fixes in existing failing test files.

## Checklist
- [x] Add/adjust tests for steward activation + evidence + compression behavior.
- [x] Implement steward activation resolution and wire through coordinator/preparation/runtime.
- [x] Add coding verify contract types + command runner and wire into shadow result metadata.
- [x] Surface evaluation report artifact paths in evaluation results and API responses.
- [x] Enrich dangerous-operation approvals with safety level / rollback / alternative hints.
- [x] Fix pre-existing lint blockers (`errcheck` / `unused`) currently failing `./dev.sh lint`.
- [x] Run full lint + full test.
- [x] Commit incrementally by module.
- [ ] Merge back to `main` (fast-forward) and remove temporary worktree.

## Validation
- `./dev.sh lint`
- `./dev.sh test`
