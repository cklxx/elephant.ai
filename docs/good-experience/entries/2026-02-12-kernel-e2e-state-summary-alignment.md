# 2026-02-12 — Kernel e2e state observability + alignment context decoupling

Impact: Kernel loop now persists actionable cycle summaries into state, refreshes system prompt snapshots each cycle, and keeps component boundaries clean (execution components consume resolved client/profile context instead of low-level config assembly).

## What changed

- Kernel cycle result now carries per-agent structured summary (`AgentSummary`) and notifier output includes per-agent completion/failure lines.
- `STATE.md` runtime block now includes `agent_summary` for each cycle, improving post-run observability.
- `SYSTEM_PROMPT.md` is refreshed each cycle via provider callback; `INIT.md` remains first-boot snapshot (seed-only).
- Added `kernel_goal` tool and kernel alignment context provider (`GOAL.md` + `SOUL.md` + `USER.md`) to inject mission/value/user service context without runtime YAML dependency for those contents.
- Kernel executor now resolves pinned LLM selection by channel/chat/user context, and memory capture prefers the same request-scoped selection when available.

## Why this worked

- Domain/app separation is preserved: kernel owns loop orchestration only; profile/client resolution and alignment context are injected by app-layer providers.
- Cycle outputs become auditable and human-readable in one place (`STATE.md` + notice text).
- Real-tool-action guard reduces false-positive “done” states.

## Validation

- `go test ./...` ✅
- `./scripts/run-golangci-lint.sh run ./...` ✅
- Real non-mock e2e run (current branch binary) succeeded and updated:
  - `~/.alex/kernel/default/STATE.md`
  - `~/.alex/kernel/default/SYSTEM_PROMPT.md`
  - `~/.alex/kernel/default/INIT.md` (unchanged seed timestamp)
