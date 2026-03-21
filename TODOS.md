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

### ~~Evaluate IM fragment quality~~ ✓ Removed
- Removed 2026-03-21. Message splitting was deleted entirely — single message per reply. The splitting hypothesis didn't pan out; Feishu renders markdown natively.

### ~~Migrate existing digest services to DigestService~~ ✓ Done
- Completed 2026-03-19. All three services (Weekly Pulse, Daily Summary, Prep Brief) now implement `digest.DigestSpec` and use `digest.Service.Run()` for the generate→format→deliver lifecycle.
