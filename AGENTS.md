# elephant.ai — Agent Overrides

> **Base rules: see `CLAUDE.md`.** This file contains additions and overrides only.

## Code review — mandatory before every commit

- **Trigger**: After lint + tests pass, before any commit or merge.
- **Entry point**: `python3 skills/code-review/run.py '{"action":"review"}'` (full workflow in `skills/code-review/SKILL.md`).
- **Blocking rule**: P0/P1 findings must be fixed before commit. P2 creates a follow-up task. P3 is optional.

## Additional agent rules

- Prefer using subagents for parallelizable tasks to improve execution speed.
- Understand the full context of changes before reviewing; respect architectural decisions over personal preferences.
