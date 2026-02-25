# Moltbook Skill Registration Plan

## Goal
Ensure Moltbook skill playbook is discoverable in server/Lark runs so "moltbook-posting" guidance is available and subagent tasks can load it reliably.

## Scope
- Skill discovery path resolution for server/Lark runs.
- Minimal config-driven or workspace-driven fallback for skills root.
- Tests for skills-dir selection helper.

## Plan
1) Add a helper to set ALEX_SKILLS_DIR from a configured workspace (if unset).
2) Invoke the helper during server startup (before coordinator use).
3) Add tests covering env override, empty workspace, missing skills dir, and success path.
4) Update docs/config notes if needed.
5) Run lint + tests.

## Progress
- 2026-02-02: Plan created.
- 2026-02-02: Added skills-dir helper + tests; wired into server startup.
- 2026-02-02: Documented ALEX_SKILLS_DIR env var in CONFIG reference.
- 2026-02-02: Ran `./dev.sh lint` and `./dev.sh test`.
