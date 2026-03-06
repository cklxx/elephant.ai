# Team / Feishu CLI Integration Backlog

Updated: 2026-03-06
Owner: eli
Worktree: /Users/bytedance/code/elephant-ai-team-integration

## Goal
把 Team / Skills / CLI / Terminal / Feishu 操作面收敛成统一产品契约，并开始首批可落地实现。

## Task Board

### P0 — Contract unification
- [in_progress] Audit product-facing docs that still describe `run_tasks` / `reply_agent` as user-facing
- [todo] Update canonical docs to state `alex team ...` is the only user-facing Team entrypoint
- [todo] Update reference docs so orchestration tools are marked internal-only
- [todo] Add explicit migration note: Team CLI-first, orchestration tools internal detail

### P1 — Feishu CLI product surface
- [todo] Define canonical `feishu-cli` skill contract
- [todo] Decide CLI namespace (`alex feishu ...` vs `alex lark ...`) with recommendation
- [todo] Map current `channel.action` surface into user jobs / product affordances
- [todo] Define approval and write-operation model

### P1 — Terminal user-visible experience
- [todo] Define Team Run view model (`team_run`, `roles`, `recent_events`, `artifacts`, `live_terminal`)
- [todo] Define minimal CLI/UI/Lark render surfaces for terminal visibility
- [todo] Specify role-level inject / follow-up UX

### P2 — Execution plumbing
- [blocked] Team task runner is currently blocked by background task capacity (`4 active (max=4)`)
- [todo] Inspect/clear stale pending background tasks or raise task pool capacity safely
- [todo] Re-run parallel team taskfile after capacity issue is resolved

## Current blockers
1. `go run ./cmd/alex team run --file ...` currently fails with:
   `background task limit reached: 4 active (max=4)`
2. Main repo is dirty; implementation continues in isolated worktree.

## Next concrete actions
1. Update project docs to align with current registry reality.
2. Draft `feishu-cli` skill spec in repo.
3. Draft Team Run / Terminal UX spec.
4. Re-check team queue status and resume parallel execution when capacity frees up.

