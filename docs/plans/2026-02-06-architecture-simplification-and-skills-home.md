# Plan: Architecture Simplification + Skills Home (2026-02-06)

## Goal
- Align roadmap execution with strict layering rules and reduce structural coupling.
- Standardize skill discovery to `ALEX_SKILLS_DIR` or `~/.alex/skills` with repo-to-home auto-sync.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.
- Started from `main` in isolated worktree branch `eli/arch-simplify-skills-home` and copied `.env`.

## Scope
1. Skills home flow: env override + default `~/.alex/skills` + copy missing repo skills.
2. Domain dependency inversion for currently leaked infra dependencies.
3. Architecture guardrails and roadmap/doc sync.

## Non-goals
- No product feature expansion.
- No external API contract changes.
- No top-level directory migration to `assets/` / `var/` in this batch.

## Execution Plan
1. Baseline and guardrail scripts.
2. Skills home implementation and web/runtime alignment.
3. Domain dependency inversion (Batch A/B/C): IDs/context, latency/json, async, working-dir/workspace.
4. Architecture checks + roadmap/document updates.
5. Full lint/test validation.
6. Incremental commits, merge back to `main`, remove worktree.

## Validation Gates
- `./dev.sh lint`
- `./dev.sh test`
- `go test ./...`
- `make check-arch`

## Progress Log
- 2026-02-06 09:20: Plan file created, baseline phase started.
