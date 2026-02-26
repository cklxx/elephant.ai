# Long-Term Memory

Updated: 2026-02-26 15:00

## Criteria
- Only keep durable knowledge that should persist across tasks.
- Prefer short, actionable statements with a clear remediation or rule.

## Topic Files
- **Eval & Routing** → see [eval-routing.md](eval-routing.md) (suite design, heuristic rules, routing patterns)
- **Kernel Operations** → see [kernel-ops.md](kernel-ops.md) (execution rules, supervisor, process management)
- **Lark & DevOps** → see [lark-devops.md](lark-devops.md) (local ops, PID management, auth infra)

## Active Memory (2026-02-26)
- Keep `agent/ports` free of memory/RAG deps; inject memory at engine/app layers to avoid import cycles.
- Config examples are YAML-only (`.yaml` paths); plans and records must follow repo conventions.
- Use TDD when touching logic; run full lint + tests before delivery.
- Context autonomy upgrades should prefer host CLI exploration (`command -v` / deterministic probes) and inject only safe, redacted environment hints (never raw secret env vars).
- Use `CGO_ENABLED=0` for `go test -race` on darwin CLT to avoid LC_DYSYMTAB warnings.
- Apply response-size caps + retention/backpressure to prevent unbounded growth.
- Long-lived per-session metrics maps must enforce cap + TTL pruning; never keep unbounded session keys.
- Streaming UI performance: dedup events, cap with LRU, RAF buffer, defer markdown parsing.
- Flush-request coalescing on async history queues must not add fixed wait to single-request paths; only coalesce when contention is present.
- Proactive hooks: hook registry + per-request MemoryPolicy; cache skills + precompile regex.
- Event partitioning: groupKey uses `parent_run_id`; subagent detection is `parent_run_id != run_id`.
- Tool event rules: only subagent `workflow.tool.started` hits main stream; others go pending/merged.
- Subagent: snapshot pruning must drop tool outputs for pruned call IDs; cap default parallelism (`maxWorkers`) and stagger starts to reduce upstream rejections.
- Memory system is Markdown-only: `~/.alex/memory/MEMORY.md` + `~/.alex/memory/memory/YYYY-MM-DD.md`.
- Bash scripts under `set -u` must guard array expansions (avoid unbound variable errors).
- Subscription model selection should remain request-scoped (web/local) and avoid mutating managed overrides YAML; persist pins in non-YAML state when needed.
- Skills resolution rule: `ALEX_SKILLS_DIR` overrides all; otherwise default `~/.alex/skills` with repo `skills/` missing-only sync and user-copy preservation.
- Meta-skills orchestration uses `configs/skills/meta-orchestrator.yaml` + skills frontmatter to gate activation and linkage.
- Keep `make check-arch` green to enforce domain import boundaries and prevent infra leakage regressions.
- **Pre-push CI gate**: `scripts/pre-push.sh` mirrors CI's fast-fail checks. Always runs before `git push`. Skip with `SKIP_PRE_PUSH=1`.
- Server restart recovery requires task persistence + startup resume hook; checkpoint-only restore is insufficient for cross-process task continuation.
- Tool SLA collection is effective only when DI wires a shared `SLACollector` into tool registry wrapping and event translation.
- Emit explicit `workflow.replan.requested` when tool failure triggers orchestrator replan to avoid UI-side inference heuristics.
- Full-chain performance tracing should include ReAct iteration + LLM generate + tool execute spans, keyed by `alex.session_id`/`alex.run_id`.
- Persona-level "always confirm" phrasing can suppress autonomy in low-risk delegated asks; add delegated-autonomy guardrails across persona and routing prompts.
- SSE attachment dedupe must avoid `json.Marshal`-based whole-object hashing on hot paths; prefer lightweight signatures.
- One-click local observability flows (`logs-ui`) should include endpoint readiness probes and targeted self-heal restart.
- Dev tools consolidation should keep diagnostics in one workbench and enforce virtualization + deferred payload expansion for large logs.
- 7C 输出优化应采用低风险整形：允许空白规范化与重复压缩，但必须保留结构化 Markdown 语义。

## Items

### Event Partitioning Architecture
- **Subagent groupKey must use `parent_run_id` first.** AgentCard render looks up by core agent's `run_id` which equals subagent's `parent_run_id`.
- **Tool event display rules:** `workflow.tool.started` in main stream only for `tool_name="subagent"`. All other tool starts go to `pendingTools` or merged into `workflow.tool.completed`. Orchestrator tools filtered from `pendingTools`.
- **`isSubagentLike` checks `parent_run_id !== run_id`** — the only reliable subagent indicator.

### Performance Patterns
- Event dedup, LRU caps, RAF buffering, deferred markdown parsing keep streaming UI responsive.
- Precompile hot-path regex, cap response reads, apply retention/backpressure to prevent unbounded growth.
- Streaming events (output deltas, tool progress, chunks) must be dropped from session history to avoid crowding terminal events.

### Project Conventions
- Config: YAML only, `.yaml` paths.
- Error experience: entries in `docs/error-experience/entries/`, summaries in `docs/error-experience/summary/entries/`; index files are index-only.
- Plans: always write to `docs/plans/`, update as work progresses.
- Record improvements in guides/entries as discovered. Commit often, prefer small commits.

### Architecture Review (2026-02-16)
- Improvement plan: `docs/plans/architecture-review-2026-02-16.md` — 4 phases: decouple → split god structs → unify events/storage → test coverage.
