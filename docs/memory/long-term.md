# Long-Term Memory

Updated: 2026-02-11 11:00

## Criteria
- Only keep durable knowledge that should persist across tasks.
- Prefer short, actionable statements with a clear remediation or rule.

## Active Memory (2026-02-10)
- Keep `agent/ports` free of memory/RAG deps; inject memory at engine/app layers to avoid import cycles.
- Config examples are YAML-only (`.yaml` paths); plans and records must follow repo conventions.
- Use TDD when touching logic; run full lint + tests before delivery.
- Context autonomy upgrades should prefer host CLI exploration (`command -v` / deterministic probes) and inject only safe, redacted environment hints (never raw secret env vars).
- Use `CGO_ENABLED=0` for `go test -race` on darwin CLT to avoid LC_DYSYMTAB warnings.
- Apply response-size caps + retention/backpressure to prevent unbounded growth.
- Long-lived per-session metrics maps (e.g., broadcaster/session counters) must enforce cap + TTL pruning; never keep unbounded session keys.
- Streaming UI performance: dedup events, cap with LRU, RAF buffer, defer markdown parsing.
- Flush-request coalescing on async history queues must not add fixed wait to single-request paths; only coalesce when contention is present.
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
- Routing pass rates can hide delivery regressions; keep a dedicated artifact-delivery collection plus sampled good/bad deliverable checks in reports.
- When foundation pass@1 gets saturated, retire repeatedly top1-perfect cases and inject conflict-heavy replacements before adding more generic volume.
- Foundation eval optimization should prioritize top1 conflict clusters (`expected => top1`) with systematic router/token convergence; keep report sections fixed with x/x scoreboard, conflict inventory, and good/bad deliverable samples.
- Motivation-aware routing benefits from dedicated conflict signals (consent-sensitive boundaries, follow-up scheduling, memory-personalized cadence) and should be validated both as standalone suite and integrated full-suite regression.
- Heuristic token matching can silently miss due to stemming normalization (e.g., trailing `s` removal like `progress` -> `progres`); add intent-level regression tests for critical conflict cases instead of token-set-only assertions.
- Hard-case expansion should map to explicit benchmark-style dimensions (sparse clue retrieval / stateful commitment boundary / reproducibility trace evidence) so failures are diagnosable and optimizable as conflict families.
- Foundation suite growth must be budgeted with explicit caps and round-level `added / retired / net` reporting to prevent silent dataset bloat.
- Hard benchmark expansion should be taxonomy-driven (benchmark family -> capability dimension -> dataset), so future retire/add decisions remain systematic instead of ad-hoc.
- Keep suite layered by hardness (Core-Hard / Frontier-Hard / Research-Frontier-Hard); add/remove cases by layer budget to maintain challenge and diagnosability.
- After first prune under a hard threshold, a second review-driven squeeze can remove residual redundancy without losing pass@5 coverage if hard stress dimensions are kept intact.
- Foundation eval reports should keep a fixed structure: x/x scoreboard (collections/cases/pass@1/pass@5/deliverable), top conflict clusters (`expected => top1`), and sampled good/bad deliverable checks with artifact paths.
- Rebuilding evaluation should be capability-layered (Foundation Core / Stateful-Memory / Delivery / Frontier Transfer) so new hard benchmark additions expose real failed cases instead of inflating easy-pass volume.
- Batch heuristic upgrades should apply exact-tool-name boosts asymmetrically: strong for specific tools, weak for generic tools (`plan`/`clarify`/`find`/`search_file`) to avoid cross-domain over-trigger regressions.
- For Lark delivery intents, explicitly separate "text-only checkpoint/no file transfer" (`lark_send_message`) from "must deliver file package" (`lark_upload_file`); otherwise upload dominates due shared chat/file vocabulary.
- For source-ingest intents, treat "single approved exact URL, no discovery" as strong `web_fetch` signal and suppress visual/browser capture tools unless proof/screenshot/UI language is explicit.
- Lark 本地链路如果 PID 文件写到包装 shell 而非真实 `alex-server`，会造成孤儿进程累积并耗尽 auth DB 连接；后台启动必须保证记录真实子进程 PID。
- auth DB 本地初始化遇到 `too many clients already` 应执行“孤儿 Lark 进程清理 + 退避重试”，比一次失败直接降级更稳定。
- DevOps `ProcessManager` 对磁盘 PID 恢复/停止不能只做 `kill(0)`；必须持久化并校验进程身份（命令行签名），避免 PID 复用误判和误杀。
- 同名进程快速替换时，旧进程 `Wait` 回调清理必须确认 map 里仍是同一实例，防止误删新进程追踪状态。
- Supervisor 重启阈值语义应统一为“达到上限触发 cooldown（>=）”，且 backoff 要异步执行，避免阻塞同一 tick 的其他组件健康处理。
- Lark loop gate 的 codex auto-fix 应默认关闭并显式开关启用（`LARK_LOOP_AUTOFIX_ENABLED=1`），否则会出现“非预期自动改代码”体验。

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
