# 2026-03-04 Agent Teams Bootstrap + Dynamic CLI

## Status
- [x] Context discovery and architecture survey
- [x] Implement team bootstrap artifacts + recovery
- [x] Implement dynamic coding CLI discovery/probe with TTL cache
- [x] Implement role capability profile + CLI selector/fallback
- [x] Bind role runtime to tmux panes with env injection
- [x] Add output/event logging and input injection pathway
- [x] Add/adjust unit tests
- [x] Run lint/tests + code review gate

## Scope
Upgrade agent teams runtime from static role dispatch to bootstrap-gated, capability-aware, resilient execution:
1. Team bootstrap artifacts (`team_id`, session dir, capability snapshot, role registry, runtime state)
2. Dynamic coding CLI detection/probe and cached capability matrix
3. Role-to-CLI selection by capability profile with fallback chain
4. tmux pane lifecycle binding and role env injection
5. Structured output/event recording and controllable input channel

## Notes
- Keep existing `codex/claude_code/kimi` path functional while introducing dynamic team runtime path.
- Do not touch unrelated files outside team/orchestration/coding/external runtime.
