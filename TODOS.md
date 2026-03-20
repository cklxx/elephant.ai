# TODOS

Updated: 2026-03-19

Items deferred from CEO Plan Reviews (2026-03-18 strategic expansion, 2026-03-19 chat+worker optimization, 2026-03-19 chat naturalness).

---

## P1 — Should do soon

### ~~Stash cleanup~~ ✓ Done
- Completed 2026-03-20. Audited 40 stashes (32 merged, 8 orphan). All orphan stashes contained trivial changes (worktree markers, stale docs, old config). Cleared all.

### Jira/Linear read connector (Signal Graph plugin)
- **What:** MCP server for reading ticket status, assignees, transitions, comments.
- **Why:** Phase 2 P0 enabler. With Signal Graph (#1 expansion), this becomes a plugin rather than a standalone integration. >95% sync success, freshness <10 min.
- **Effort:** M (human: ~1 week / CC: ~30 min)
- **Depends on:** Signal Graph architecture being in place. **Done — `internal/app/signals/`**
- **Code path:** `internal/infra/tools/builtin/jira/` + signal source adapter.

### ~~Git/GitHub signal hardening~~ ✓ Done
- Completed 2026-03-20. Added GitHub webhook receiver (`POST /api/webhooks/github`) with HMAC-SHA256 verification and event sink. Extended event normalization: `review_requested` action, `CreateEvent`/`DeleteEvent` for branches, `ReviewState` properly propagated from review events. Webhook secret configurable via `GitHubConfig.WebhookSecret`.

### ~~Cross-task awareness (Chat + Worker phase 3)~~ ✓ Done
- Completed 2026-03-19. `lastResultPreview` stored in sessionSlot on task completion (≤200 runes). `resolveTaskReferences` scans dispatch task descriptions for `#N` patterns and prepends referenced results as context. Worker snapshots expose completed task results to the conversation LLM.

### ~~Memory-wired formality level~~ ✓ Done
- Completed 2026-03-19. `detectFormalityLevel` now scans cached memory context for relationship keywords (外部客户/client → neutral, 同事/colleague → casual) before falling back to chat type heuristic.

## P2 — Should do before next milestone

### ~~ARCHITECTURE.md diagram update~~ ✓ Done
- Completed 2026-03-18. New components documented: Signal Graph, Decision Engine, Memory Distillation, Unified Session Store.

### Evaluate IM fragment quality (2-week post-ship)
- **What:** After 2 weeks of server-side splitting, evaluate whether mechanical clause-boundary splitting feels natural enough or whether JSON structured output (LLM-controlled fragment boundaries) is worth the added complexity.
- **Why:** Server-side splitting is a hypothesis — real user conversations may reveal that the LLM should control where breaks go, not a regex scanner.
- **Effort:** S (human: ~2h / CC: ~10 min)
- **Depends on:** IM fragmented replies PR shipping and running for 2 weeks.

### ~~Migrate existing digest services to DigestService~~ ✓ Done
- Completed 2026-03-19. All three services (Weekly Pulse, Daily Summary, Prep Brief) now implement `digest.DigestSpec` and use `digest.Service.Run()` for the generate→format→deliver lifecycle.
