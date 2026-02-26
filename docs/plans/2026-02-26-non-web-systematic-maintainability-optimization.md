# 2026-02-26 Non-Web Systematic Maintainability Optimization

## Objective

Systematically improve maintainability, readability, and logic simplicity across the repository excluding `web/`, using low-risk, behavior-preserving refactors driven by evidence (duplication/complexity/hotspot scans + existing project memory).

## Best-Practice Basis

- Local engineering guide: `docs/guides/engineering-practices.md`
- Skill SOP: `skills/best-practice-search/SKILL.md`
- Prior optimization rounds:
  - `docs/plans/2026-02-24-continue-full-optimization.md`
  - `docs/plans/2026-02-24-continue-full-optimization-r2.md`
  - `docs/plans/2026-02-24-continue-full-optimization-r3.md`
- Active memory focus:
  - keep boundary checks centralized
  - reduce hot-path log/noise duplication
  - avoid unbounded structures and repeated payload inflation

## Scope

1. `internal/` + `cmd/` + `scripts/` only; exclude `web/`.
2. Prefer cross-cutting simplification patterns over isolated cosmetic edits.
3. No compatibility shims; clean redesign from first principles for touched slices.

## Execution Plan

- [completed] Baseline scan: repo hotspots, duplication, complexity, TODO/FIXME signals.
- [completed] Candidate ranking: picked high-leverage, low-risk refactors across DI/config/lark/CLI layers.
- [completed] Implementation: applied simplifications with focused tests.
- [completed] Validation: ran lint/tests/arch checks.
- [in_progress] Mandatory code review + incremental commits.

## Progress Log

- 2026-02-26: Loaded engineering guide, best-practice skill SOP, and memory summaries before implementation.
- 2026-02-26: Completed pre-work checklist on `main` (`git diff --stat`, `git log --oneline -10`, suspicious-change review).
- 2026-02-26: Initialized systematic non-web optimization plan and started baseline scan.
- 2026-02-26: Baseline findings (non-web): high duplication in Lark pagination paths, duplicated DI app-config assembly, duplicated HTTP limit application, and high-complexity config/CLI core functions.
- 2026-02-26: Refactored DI core assembly: extracted shared `buildAgentAppConfig()` and removed duplicated struct literals in primary/alternate coordinator construction.
- 2026-02-26: Refactored config core path: rewrote `applyLarkConfig` into composable helpers (`applyLarkBrowserConfig`, `applyLarkPersistenceConfig`, typed assignment helpers) to reduce branch density while preserving behavior.
- 2026-02-26: Refactored CLI override core path: replaced large switch-based `setOverrideField`/`clearOverrideField` with handler registry + typed parsers for maintainability.
- 2026-02-26: Consolidated HTTP limit application via shared typed helper (`applyHTTPLimitsValues`) across file/override paths.
- 2026-02-26: Consolidated Lark pagination boilerplate with shared helpers (`normalizePageSize`, `extractPageTokenAndHasMore`) and applied across multiple services.
- 2026-02-26: Ran targeted tests for changed packages and full quality gate (`./scripts/pre-push.sh`) successfully.
- 2026-02-26: Ran mandatory code-review skill collection and performed manual blocking review; no P0/P1 found in this change set.
