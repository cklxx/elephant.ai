# TODOS

Updated: 2026-03-18

Items deferred from the CEO Plan Review (2026-03-18 strategic expansion).

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
- **Depends on:** Signal Graph architecture being in place.
- **Code path:** `internal/infra/tools/builtin/jira/` + signal source adapter.

### Git/GitHub signal hardening
- **What:** Harden existing Git MCP for PRs, commits, review status, deploy events. Add webhook receiver for Signal Graph integration.
- **Why:** Phase 2 P0 enabler. Existing MCP needs webhook endpoint for real-time events vs polling.
- **Effort:** S (human: ~3 days / CC: ~20 min)
- **Depends on:** Signal Graph architecture being in place.
- **Code path:** `internal/infra/tools/builtin/git/` + signal source adapter.

## P2 — Should do before next milestone

### ARCHITECTURE.md diagram update
- **What:** Update architecture diagrams after the 6 expansion features land. New components: Signal Graph, Decision Engine, Unified Session Store, Memory Distillation.
- **Why:** Stale diagrams are worse than none. The expansion adds 4 major components to the architecture.
- **Effort:** S (human: ~2 hours / CC: ~15 min)
- **Depends on:** At least one expansion track landing.

### Migrate existing digest services to DigestService
- **What:** Refactor Weekly Pulse (`internal/app/pulse/weekly.go`), Daily Summary (`internal/app/summary/daily.go`), and Prep Brief (`internal/app/prepbrief/brief.go`) to use the shared DigestService abstraction.
- **Why:** All 3 follow the same gather→format→deliver pattern. After DigestService is built for Morning Brief + Self-Report, migrating the existing 3 eliminates duplicated scaffolding.
- **Effort:** S (human: ~2 days / CC: ~20 min)
- **Depends on:** DigestService abstraction being built (part of expansion scope).
- **Code path:** `internal/app/{pulse,summary,prepbrief}/` → use `internal/app/digest/`.
