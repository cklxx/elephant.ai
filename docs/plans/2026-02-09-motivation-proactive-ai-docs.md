# Plan: Human motivation report, proactive-AI evaluation set, and heuristic prompt updates

Owner: cklxx
Date: 2026-02-09
Worktree: `.worktrees/motivation-report-20260209`
Branch: `cklxx/motivation-report-and-heuristics-20260209`

## Goal
Convert the prior motivation research into repository docs that are executable for product/evaluation work, then add English heuristic prompt guidance to `CLAUDE.md` and `AGENTS.md`.

## Active Memory Set (Top 10)
Ranked by recency/frequency/relevance to this task.

1. Start from `main` in a new worktree branch and copy `.env`, then merge back and clean temporary worktree.
2. Non-trivial tasks require a tracked plan under `docs/plans/` with progress updates.
3. Config examples stay YAML-only (`.yaml` paths).
4. Run full lint + tests before delivery; use TDD for logic changes.
5. Mandatory code review before commit/merge, using `skills/code-review/SKILL.md` and referenced checklists.
6. Commit every completed solution in incremental commits.
7. Keep records discipline: entries under `docs/error-experience/entries` and summaries under `.../summary/entries`; index files remain index-only.
8. For evaluation quality, favor layered suites (tool coverage / prompt effectiveness / proactivity / complex tasks) over one mixed suite.
9. For pass@1 optimization, use targeted conflict-pair disambiguation and stage-aware routing rather than broad token boosts.
10. Avoid risky/destructive repo operations and history rewrites.

## Scope
- Add a complete report doc: motivation-source model + proactive-AI application patterns.
- Add an evaluation-set design doc and runnable dataset YAML for motivation/proactivity cases.
- Add an explicit validation methodology (offline + online + safety/ethics checks).
- Add English heuristic prompt snippets to `CLAUDE.md` and `AGENTS.md`.

## Steps
- [x] Load practices and memory sources; derive active memory set.
- [x] Draft report and evaluation/validation docs.
- [x] Add/validate evaluation dataset YAML wiring.
- [x] Update `CLAUDE.md` and `AGENTS.md` with English heuristic prompts.
- [x] Run lint/tests for impacted scope.
- [x] Execute mandatory code review flow and address findings.
- [ ] Commit incrementally, merge to `main`, remove temporary worktree.

## Progress Log
- 2026-02-09 13:00: Created worktree from `main`, copied `.env`, loaded engineering practices and latest memory entries/summaries.
- 2026-02-09 13:15: Added research + analysis docs for motivation source model, proactive application, evaluation-set design, and validation workflow.
- 2026-02-09 13:18: Added standalone motivation-aware foundation suite YAML and inserted English heuristic prompting sections into `CLAUDE.md` and `AGENTS.md`.
- 2026-02-09 13:34: Ran `./dev.sh lint` and `./dev.sh test`; full test rerun hit existing flaky `TestTickRestartBackoffIsAsync`, immediate targeted rerun passed.
- 2026-02-09 13:36: Completed mandatory code review workflow; no P0-P3 findings.
