## Goal

Replace JSON-based skill invocation/output contracts with CLI-style argument and text output contracts across the repository, prioritized by runtime impact.

## Scope

- `skills/*/run.py`
- `skills/*/SKILL.md`
- `internal/infra/skills/*`
- `internal/infra/tools/builtin/session/*`
- `scripts/skill_runner/*`
- `evaluation/skills_e2e/*`
- relevant tests

## Priority Plan

1. P0 Runtime contract migration
   - Convert all `skills/*/run.py` entrypoints from JSON payload parsing to CLI argument parsing.
   - Convert default output from JSON object serialization to CLI text output.
2. P1 Agent/docs command surface migration
   - Update skill docs and generated/exposed skill command examples to CLI style.
3. P2 Validation/tooling migration
   - Update smoke/e2e generators, datasets, and tests that currently enforce JSON command/response.
4. P3 Verification and cleanup
   - Run targeted tests and fix regressions.
   - Summarize migrated counts and residual risks.

## Progress

- [x] Pre-work checklist on `main` completed; existing unrelated dirty changes flagged.
- [x] Worktree created: `../elephant.ai-wt-skills-cli-format` on branch `fix/skills-cli-format`.
- [x] JSON-usage inventory completed with subagents.
- [x] P0 runtime migration implemented.
- [x] P1 docs/surface migration implemented.
- [x] P2 tooling/tests migration implemented.
- [x] Verification complete.
