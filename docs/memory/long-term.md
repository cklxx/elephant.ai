# Long-Term Memory

Updated: 2026-02-04 10:00

## Criteria
- Only keep durable knowledge that should persist across tasks.
- Prefer short, actionable statements with a clear remediation or rule.

## Active Memory (2026-02-04)
- Keep `agent/ports` free of memory/RAG deps; inject memory at engine/app layers to avoid import cycles.
- Config examples are YAML-only (`.yaml` paths); plans and records must follow repo conventions.
- Use TDD when touching logic; run full lint + tests before delivery.
- Use `CGO_ENABLED=0` for `go test -race` on darwin CLT to avoid LC_DYSYMTAB warnings.
- Apply response-size caps + retention/backpressure to prevent unbounded growth.
- Streaming UI performance: dedup events, cap with LRU, RAF buffer, defer markdown parsing.
- Proactive hooks: hook registry + per-request MemoryPolicy; cache skills + precompile regex.
- Event partitioning: groupKey uses `parent_run_id`; subagent detection is `parent_run_id != run_id`.
- Tool event rules: only subagent `workflow.tool.started` hits main stream; others go pending/merged.
- Subagent: snapshot pruning must drop tool outputs for pruned call IDs; cap default parallelism (`maxWorkers`) and stagger starts to reduce upstream rejections.
- Memory system is Markdown-only: `~/.alex/memory/MEMORY.md` + `~/.alex/memory/memory/YYYY-MM-DD.md`.
- Bash scripts under `set -u` must guard array expansions (avoid unbound variable errors).

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
