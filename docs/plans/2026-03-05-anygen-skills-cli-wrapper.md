# 2026-03-05 · AnyGen Skills CLI Wrapper (Progressive Disclosure)

## Goal

Wrap `https://github.com/AnyGenIO/anygen-skills` into elephant.ai's local `skill + run.py` style, with a unified CLI runtime and progressive help disclosure for agent self-discovery.

## Constraints

- Preserve existing `skills/anygen-task-creator` behavior (no breaking migration in this task).
- Follow repository skill conventions: frontmatter + `run.py` JSON in/out.
- Progressive disclosure must support `help -> modules -> module -> action`.

## Plan

1. `completed` Add `scripts/cli/anygen/anygen_cli.py` + `scripts/skill_runner/anygen_cli.py` with:
   - command dispatcher: `help | task`
   - progressive help topics: `overview | modules | module | action`
   - task actions passthrough: `create | status | poll | download | run`
2. `completed` Add `skills/anygen/SKILL.md` and `skills/anygen/run.py`.
3. `completed` Add tests for runtime + runner + skill wrapper.
4. `completed` Run targeted tests and repository lint/test gates.
5. `completed` Record experience entries and finalize commits.

## Progress Log

- 2026-03-05 14:59 +08:00: Loaded memory/guides, reviewed existing skill architecture, inspected upstream AnyGen skills repository.
- 2026-03-05 15:03 +08:00: Implemented unified AnyGen CLI runtime (`help/task`) with progressive help topics and task-manager execution actions.
- 2026-03-05 15:04 +08:00: Added new `skills/anygen` wrapper and progressive SKILL.md.
- 2026-03-05 15:05 +08:00: Added tests; targeted pytest passed (`12 passed`).
- 2026-03-05 15:14 +08:00: Ran `make dev-lint` and `make dev-test` successfully in worktree.
- 2026-03-05 15:15 +08:00: Added good-experience entry + summary; updated `docs/memory/user-patterns.md` and ran memory graph backfill.
