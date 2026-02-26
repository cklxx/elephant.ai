# Plan: Non-web Systemic Maintainability Optimization (2026-02-26)

## Goal
- Scan and optimize code outside `web/` from a systemic perspective:
  - improve maintainability and readability,
  - simplify logic and reduce duplication,
  - preserve behavior with focused tests.

## Scope
- Included: `cmd/`, `internal/`, `configs/`, `scripts/`, `skills/` (code and related tests when touched).
- Excluded: `web/`.
- Existing unrelated dirty files are protected and must not be overwritten.

## Best-practice anchors used
- `skills/best-practice-search/SKILL.md`: use "检索→筛选→适配" flow and produce actionable, bounded changes.
- `docs/guides/engineering-practices.md`: small reviewable changes, TDD when touching logic, full lint/tests before delivery.
- Go style anchors in local guide: `Effective Go`, `Go Code Review Comments`, `Uber Go Style`.

## Active memory set (first-run load, 2026-02-26)
- `err-2026-02-25-kernel-context-leak`: runtime-boundary gating must have positive/negative tests.
- `errsum-2026-02-10-tool-optimization-caused-eval-tool-availability-collapse`: avoid broad refactor that breaks capability surface.
- `err-2026-02-12-go-sum-stale-ci-failure-no-prepush-gate`: keep verification chain strict before delivery.
- `good-2026-02-24-systematic-log-noise-reduction`: remove redundant hot-path noise while preserving diagnostics.
- `good-2026-02-25-unattended-boundary-guard`: localize guard at a clear boundary and encode dual-path tests.
- `goodsum-2026-02-12-llm-profile-client-provider-decoupling`: keep layering clean and reduce coupling.
- `goodsum-2026-02-23-branch-delete-policy-fallback`: prefer safe fallback commands if policy blocks porcelain.
- `ltm-long-term-memory`: maintain `agent/ports` boundaries and keep changes incremental.

## One-hop neighbors (relevance-ranked, max 6)
- `errsum-2026-02-25-kernel-context-leak`
- `goodsum-2026-02-25-unattended-boundary-guard-for-kernel-context`
- `goodsum-2026-02-24-systematic-log-noise-reduction`
- `good-2026-02-24-systematic-log-noise-reduction`
- `good-2026-02-12-llm-profile-client-provider-decoupling`
- `goodsum-2026-02-12-llm-profile-client-provider-decoupling`

## Execution checklist
- [x] Pre-work checklist on `main` done (`git diff --stat`, `git log --oneline -10`, suspicious changes flagged).
- [x] Best-practice skill and engineering guides reviewed.
- [x] Repo-wide non-web hotspot scan (complexity, duplication, readability smells).
- [x] Implement 2-4 high-impact simplifications with tests.
- [x] Run full lint + tests (or document blockers).
- [x] Run mandatory code review workflow and fix P0/P1.
- [ ] Commit incremental changes.
- [ ] Update plan with final validation notes.

## Progress log
- 2026-02-26 00:00: Created plan, loaded active memory, and locked scope to non-web paths.
- 2026-02-26 00:10: Completed hotspot scan across non-web Go code; selected `internal/shared/config` duplication as highest-impact low-risk target.
- 2026-02-26 00:25: Refactored `applyFile` and `applyOverrides` into grouped field-application patterns to reduce repetitive branching while preserving source-tracking behavior.
- 2026-02-26 00:30: Simplified preset validation (`IsValidPreset`) to reuse `GetPromptConfig`, removing duplicate preset lists.
- 2026-02-26 00:35: Validation passed via targeted tests and full `./scripts/pre-push.sh` chain.
