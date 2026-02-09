# Long-Term Memory

Updated: 2026-02-09 10:00

## Criteria
- Only keep durable knowledge that should persist across tasks.
- Prefer short, actionable statements with a clear remediation or rule.

## Active Memory (2026-02-09)
- Keep `agent/ports` free of memory/RAG deps; inject memory at engine/app layers to avoid import cycles.
- Config examples are YAML-only (`.yaml` paths); plans and records must follow repo conventions.
- Use TDD when touching logic; run full lint + tests before delivery.
- Use `CGO_ENABLED=0` for `go test -race` on darwin CLT to avoid LC_DYSYMTAB warnings.
- Apply response-size caps + retention/backpressure to prevent unbounded growth.
- Streaming UI performance: dedup events, cap with LRU, RAF buffer, defer markdown parsing.
- Proactive hooks: hook registry + per-request MemoryPolicy; cache skills + precompile regex.
- Event partitioning: groupKey uses `parent_run_id`; subagent detection is `parent_run_id != run_id`.
- Tool event rules: only subagent `workflow.tool.started` hits main stream; others go pending/merged.
- Lark `/model use --chat` should resolve at chat scope first (`channel+chat_id`) with legacy `chat+user` compatibility fallback, otherwise group chats can miss pinned credentials.
- Subagent: snapshot pruning must drop tool outputs for pruned call IDs; cap default parallelism (`maxWorkers`) and stagger starts to reduce upstream rejections.
- Memory system is Markdown-only: `~/.alex/memory/MEMORY.md` + `~/.alex/memory/memory/YYYY-MM-DD.md`.
- Bash scripts under `set -u` must guard array expansions (avoid unbound variable errors).
- Subscription model selection should remain request-scoped (web/local) and avoid mutating managed overrides YAML; persist pins in non-YAML state when needed.
- Skills resolution rule: `ALEX_SKILLS_DIR` overrides all; otherwise default `~/.alex/skills` with repo `skills/` missing-only sync and user-copy preservation.
- Keep `make check-arch` green to enforce domain import boundaries and prevent infra leakage regressions.
- Lark callbacks: `channels.lark` supports `${ENV}` expansion; callback token/encrypt key also have env fallback keys in bootstrap to avoid silent callback disablement.
- Server restart recovery requires task persistence + startup resume hook; checkpoint-only restore is insufficient for cross-process task continuation.
- Tool SLA collection is effective only when DI wires a shared `SLACollector` into tool registry wrapping and event translation.
- Emit explicit `workflow.replan.requested` when tool failure triggers orchestrator replan to avoid UI-side inference heuristics.
- Full-chain performance tracing should include ReAct iteration + LLM generate + tool execute spans, keyed by `alex.session_id`/`alex.run_id` for cross-request correlation.
- SSE attachment dedupe must avoid `json.Marshal`-based whole-object hashing on hot paths; prefer lightweight signatures and hash payload only when needed.
- One-click local observability flows (`logs-ui`) should include endpoint/page readiness probes and targeted self-heal restart to avoid stale-process false alarms.
- Dev tools consolidation should keep diagnostics workloads in one workbench and enforce virtualization + deferred payload expansion for large logs.
- Server auth config should support env fallback for JWT/OAuth fields; in development-like environments auth DB failures should degrade to memory stores instead of disabling the whole auth module.
- To reduce auth session-loss regressions under multi-process dev load, cap auth DB pool connections with `auth.database_pool_max_conns` / `AUTH_DATABASE_POOL_MAX_CONNS` (default `4`).
- Foundation eval hardening works best when availability errors are explicit (`availability_error`) and heuristic routing uses action+object dual-condition gating to prevent broad-token regressions.
- Layered foundation suites (tool coverage / prompt effectiveness / proactivity / complex tasks) make routing regressions diagnosable faster than a single mixed case set.
- A dedicated memory-capabilities collection catches regressions in memory_search/memory_get and memory-informed execution chains earlier than mixed suites.
- User-habit and soul continuity routing requires separate evaluation layer; otherwise preference/persona regressions are hidden by generic memory pass rates.
- A dedicated speed-focused collection is useful to catch regressions where the router drifts to slower multi-step paths instead of shortest viable completion.

## Items

### Event Partitioning Architecture
- **Subagent groupKey must use `parent_run_id` first.** `causation_id`/`call_id` are per-tool-call within the subagent and fragment events into orphan groups. AgentCard render looks up by core agent's `run_id` which equals subagent's `parent_run_id`.
- **Tool event display rules:** `workflow.tool.started` in main stream only for `tool_name="subagent"` (renders AgentCard). All other tool starts go to `pendingTools` (running) or are merged into `workflow.tool.completed` (done). Orchestrator tools (`plan`/`clarify`) are filtered from `pendingTools`.
- **`isSubagentLike` checks `parent_run_id !== run_id`** — the only reliable subagent indicator. Do NOT rely on `agent_level`.
- **Debug technique for event routing:** Add a single aggregated `console.log` at the end of `partitionEvents` dumping all classification decisions. One log > many scattered logs.

### Session & Streaming
- Streaming events (output deltas, tool progress, chunks) must be dropped from session history to avoid crowding terminal events.
- Subagent card ordering uses anchor-based injection with `call_id` or `parent_task_id`/`task_id` for causal ordering.

### Performance Patterns
- Event dedup, LRU caps, RAF buffering, deferred markdown parsing keep streaming UI responsive.
- Precompile hot-path regex, cap response reads, apply retention/backpressure to prevent unbounded growth.
- Markdown memory is durable; avoid bloating logs and keep long-term notes concise.

### Project Conventions
- Config: YAML only, `.yaml` paths.
- Error experience: entries in `docs/error-experience/entries/`, summaries in `docs/error-experience/summary/entries/`; index files are index-only.
- Plans: always write to `docs/plans/`, update as work progresses.
- Continuously review best practices and execution flow; record improvements in guides/entries as they are discovered.
- Commit often, prefer small commits. Run full lint+tsc after changes.
- Keep `agent/ports` free of memory/RAG dependencies; inject memory service at engine/app layers to avoid import cycles.
