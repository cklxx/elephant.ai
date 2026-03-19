# TODOS

Updated: 2026-03-19

Items deferred from CEO Plan Reviews (2026-03-18 strategic expansion, 2026-03-19 chat+worker optimization, 2026-03-19 chat naturalness).

---

## P1 — Should do soon

### Stash cleanup
- **What:** Audit and clean up 39 git stashes, many >2 months old.
- **Why:** Stale stashes accumulate confusion and suggest unfinished work streams. Some may contain useful WIP that should be branches or discarded.
- **Effort:** S (human: ~1 hour / CC: ~15 min)
- **Depends on:** Nothing.

### Jira/Linear read connector (Signal Graph plugin)
- **What:** MCP server for reading ticket status, assignees, transitions, comments.
- **Why:** Phase 2 P0 enabler. With Signal Graph (#1 expansion), this becomes a plugin rather than a standalone integration. >95% sync success, freshness <10 min.
- **Effort:** M (human: ~1 week / CC: ~30 min)
- **Depends on:** Signal Graph architecture being in place. **Done — `internal/app/signals/`**
- **Code path:** `internal/infra/tools/builtin/jira/` + signal source adapter.

### Git/GitHub signal hardening
- **What:** Harden existing Git MCP for PRs, commits, review status, deploy events. Add webhook receiver for Signal Graph integration.
- **Why:** Phase 2 P0 enabler. Existing MCP needs webhook endpoint for real-time events vs polling.
- **Effort:** S (human: ~3 days / CC: ~20 min)
- **Depends on:** Signal Graph architecture being in place. **Done — `internal/app/signals/`**
- **Code path:** `internal/infra/tools/builtin/git/` + signal source adapter.

### Cross-task awareness (Chat + Worker phase 3)
- **What:** Allow a dispatched task to reference the output of a previous task by ID — "use what task #1 found as input for task #2."
- **Why:** Unlocked by task IDs from the 2026-03-19 multi-worker plan. Without this, multi-worker is parallel but not coordinated. With it, elephant.ai can run sequential pipelines expressed in natural language.
- **Effort:** M (human: ~1 week / CC: ~30 min)
- **Depends on:** Task ID tracking (N concurrent workers plan — scheduled next sprint). Requires task result storage: a per-slot result field populated on worker completion.
- **Code path:** `internal/delivery/channels/lark/` — slot result store + Chat system prompt update + `dispatch_worker` tool param `depends_on: ["#1"]`.

### Memory-wired formality level
- **What:** Extend `detectFormalityLevel` in `conversation_process.go` to scan USER.md/SOUL.md for relationship signals: keywords like "外部客户" or "client" → level=0 (neutral), "同事" or "colleague" → level=1 (casual). Currently uses chat type as a proxy (p2p=casual, group=neutral).
- **Why:** The p2p heuristic is correct most of the time but misses group chats with known colleagues. Memory-wired detection would make the formality dial genuinely per-relationship.
- **Effort:** S (human: ~2 days / CC: ~10 min)
- **Depends on:** Memory engine identity loading — already wired via `conversationPromptLoader`.
- **Code path:** `internal/delivery/channels/lark/conversation_process.go` — `detectFormalityLevel` + `handleViaConversationProcess`.

## P2 — Should do before next milestone

### ~~ARCHITECTURE.md diagram update~~ ✓ Done
- Completed 2026-03-18. New components documented: Signal Graph, Decision Engine, Memory Distillation, Unified Session Store.

### Migrate existing digest services to DigestService
- **What:** Refactor Weekly Pulse (`internal/app/pulse/weekly.go`), Daily Summary (`internal/app/summary/daily.go`), and Prep Brief (`internal/app/prepbrief/brief.go`) to use the shared DigestService abstraction.
- **Why:** All 3 follow the same gather→format→deliver pattern. After DigestService is built for Morning Brief + Self-Report, migrating the existing 3 eliminates duplicated scaffolding.
- **Effort:** S (human: ~2 days / CC: ~20 min)
- **Depends on:** DigestService abstraction being built. **Done — `internal/app/digest/`**
- **Code path:** `internal/app/{pulse,summary,prepbrief}/` → use `internal/app/digest/`.
