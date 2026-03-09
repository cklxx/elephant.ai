# Runtime & Events — Long-Term Memory Topic

Updated: 2026-03-09 12:00

Extracted from `long-term.md` to keep the main file concise.

---

## Event Partitioning
- groupKey uses `parent_run_id`; subagent detection: `parent_run_id != run_id`.
- Tool event display: only subagent `workflow.tool.started` hits main stream; others go pending/merged.
- Emit explicit `workflow.replan.requested` on tool failure replan (no UI-side inference).

## Streaming & Performance
- Dedup events, cap with LRU, RAF buffer, defer markdown parsing for streaming UI.
- Precompile hot-path regex, cap response reads, apply retention/backpressure.
- Streaming events (deltas, tool progress, chunks) must be dropped from session history.
- SSE attachment dedupe: lightweight signatures, not `json.Marshal` whole-object hashing.
- Response-size caps + retention to prevent unbounded growth.
- Per-session metrics maps: enforce cap + TTL pruning; never unbounded session keys.

## Subagent
- Snapshot pruning must drop tool outputs for pruned call IDs.
- Cap default parallelism (`maxWorkers`) and stagger starts to reduce upstream rejections.

## Proactive & Skills
- Proactive hooks: hook registry + per-request MemoryPolicy; cache skills + precompile regex.
- Meta-skills orchestration: `configs/skills/meta-orchestrator.yaml` + frontmatter gating.
- Persona-level "always confirm" can suppress autonomy; add delegated-autonomy guardrails.
- Flush-request coalescing: only coalesce when contention is present, no fixed wait on single-request paths.

## Infrastructure
- Server restart recovery requires task persistence + startup resume hook.
- Tool SLA collection: DI must wire shared `SLACollector` into tool registry wrapping.
- Full-chain tracing: ReAct iteration + LLM generate + tool execute spans, keyed by session/run ID.
- Dev tools consolidation: single workbench, virtualization + deferred payload expansion.
- One-click observability (`logs-ui`): endpoint readiness probes + self-heal restart.
- 7C output: low-risk shaping (whitespace normalization, dedup) preserving structured Markdown semantics.
- Context autonomy: prefer host CLI exploration (`command -v`); inject only safe, redacted env hints.
