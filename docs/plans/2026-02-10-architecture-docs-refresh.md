# Plan: Architecture Review and Non-Record Docs Refresh

> Created: 2026-02-10
> Status: completed
> Trigger: User requested to review current architecture and update all non-record documentation.

## Goal & Success Criteria
- **Goal**: Align non-record documentation with the current code architecture, boundaries, runtime flow, and operational commands.
- **Done when**:
  - Architecture-facing docs reflect current layering, key modules, and integration flow.
  - Non-record docs under `docs/` and root-level product docs are synchronized with current implementation facts.
  - Record-style docs (error/good experience entries and summaries) remain unchanged.
  - Lint/tests relevant to docs changes and consistency checks pass.
- **Non-goals**:
  - Changing product behavior or runtime logic.
  - Editing incident/win record entries except adding links if required by index format.

## Current State
- The repo has recent changes in eval/tooling/process-manager areas that likely shifted architecture emphasis.
- Documentation appears distributed across root docs and `docs/` subtrees, with mixed update recency.
- Record docs follow strict index/entry conventions and must be excluded from bulk refresh.

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | Build doc scope matrix (record vs non-record) | `docs/`, root `*.md` | S | — |
| 2 | Review current architecture from code and existing architecture docs | `internal/`, `cmd/`, `docs/reference/` | M | T1 |
| 3 | Update architecture core docs | `docs/reference/*.md`, `README*.md`, `ROADMAP.md` | M | T2 |
| 4 | Update non-record operational/design docs impacted by architecture | selected `docs/**/*.md` excluding record trees | M | T2 |
| 5 | Validate consistency and summarize | plan file + lint/test commands | S | T3,T4 |

## Technical Design
- **Approach**:
  - Create a strict exclusion list for record-only docs (`docs/error-experience/**`, `docs/good-experience/**`).
  - Use code-as-source-of-truth: inspect `cmd/`, `internal/app/`, `internal/domain/`, `internal/infra/`, `internal/delivery/`, and `web/` entry points and wiring.
  - Apply targeted documentation updates rather than wholesale rewrites, preserving stable sections while fixing stale architecture statements, paths, commands, and component ownership.
  - Keep changes reviewable with incremental commits grouped by doc concern.
- **Alternatives rejected**:
  - Full rewrite of all docs from scratch (high risk of introducing inaccuracies and unnecessary churn).
  - Minimal README-only update (insufficient for “all non-record docs” request).
- **Key decisions**:
  - Treat all docs outside record trees as candidate scope, then update only files with architecture drift.
  - Include plan/memory metadata refreshes required by repo conventions in this same branch.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Over-updating stable docs with no factual drift | M | M | Use code references and diff-based justification for each edit |
| Missing a stale statement in deep docs | M | M | Build and check an explicit scope matrix before edits |
| Inconsistent terminology across docs | H | M | Standardize module names/layers from current package structure |

## Verification
- Run docs-facing checks (`markdown`/link checks if available) and repo-standard lint/tests.
- Manually spot-check key paths/commands mentioned in updated docs.
- `git diff --stat` review to ensure only non-record docs and planned metadata changed.
- Rollback plan: revert specific doc commits if any statement proves incorrect.

## Progress
- 2026-02-10 12:10: Completed scope matrix and architecture fact review from `cmd/*`, `internal/app/di`, `internal/app/toolregistry`, `internal/domain/agent`, and delivery packages.
- 2026-02-10 12:25: Rewrote core architecture/ops/reference docs and refreshed indexes for non-record documentation.
- 2026-02-10 12:30: Running consistency checks, lint/tests, and mandatory code review workflow before commit.
- 2026-02-10 12:45: Mandatory code review completed (P0/P1 none; fixed P2 doc-accuracy items in `docs/reference/CONFIG.md` and `docs/reference/LOG_FILES.md`), then re-ran `make fmt` and `make test` successfully.
