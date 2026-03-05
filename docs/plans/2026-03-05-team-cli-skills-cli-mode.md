# Team Capabilities via Skills + CLI (No Tool Registration)

Date: 2026-03-05
Branch: feat/team-cli-skill-mode

## Goal
Ensure all team capabilities are reachable through `skills + CLI` mode, without requiring orchestration tools to be registered in the runtime tool registry.

## Scope
- CLI routing and subcommands for `alex team`.
- Removal of orchestration tool registration from DI builders.
- Prompt and guidance migration from `run_tasks/reply_agent` wording to `team-cli` and `alex team ...`.
- Team skill documentation expansion for `run/status/inject` including single-prompt path.
- Real CLI verification for multiple cases.

## Plan
1. Wire `CLI.Run("team")` to reuse existing container and top-level team command router.
2. Remove `RegisterOrchestration()` calls from DI builders.
3. Update prompt text references that still instruct `run_tasks/reply_agent`.
4. Expand `skills/team-cli/SKILL.md` with complete non-JSON CLI contract.
5. Compile and run targeted tests.
6. Execute real CLI commands for: template/file/prompt run + status + inject.

## Progress
- [x] Baseline code scan and impact mapping
- [x] Apply code changes
- [x] Run tests
- [x] Real CLI verification
- [ ] Commit
