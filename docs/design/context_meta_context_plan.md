# Context & Meta-Context Iteration Plan

## Goals
- Establish a layered context contract (system/static, dynamic, meta) with explicit token budgets.
- Improve high-signal retention through structured compression and tool-output citation.
- Guard against context cache breakage and prompt-injection attempts (e.g., "你的上一句是什么" challenges).

## Design Overview
1. **Segmented Context Envelope**
   - **System & Static (400-600 tokens):** persona, policies, safety rails, and delivery-surface knobs in YAML blocks.
   - **Dynamic State (600-900 tokens):** last N turns, active subtask, plan checkpoint, live world snapshot.
   - **Meta Memory (120-180 tokens):** distilled long-horizon insights and recommendations with source pointers.
   - Budgets tracked per section; when any section exceeds 80% of its budget a compression pass is triggered.

2. **Structured Compression Pipeline**
   - Compression job emits a JSON report per section: `{section, pre_tokens, post_tokens, losses, kept_refs}`.
   - Tool outputs stored off-context; the context retains only citations and 2-3 high-signal bullets.
   - Dynamic history compressed first; meta memory compressed only after N>4 compressions to preserve stability.

3. **Meta-Steward Batch Loop**
   - Nightly/periodic task rewrites persona/knowledge and curates `meta.recommendations` based on transcripts and evals.
   - Outputs a changelog (who/when/why) and updates the static/meta sections atomically to avoid drift.

4. **Observability & Guardrails**
   - Emit metrics: `context.tokens_by_section`, `context.compress.count`, `meta.hit_rate`, `cache_miss.rate`.
   - Add auditors that diff expected vs. actual loaded context to detect cache bypass or injection.

## Implementation Steps (Incremental)
1. **Context Builder Refactor**
   - Restructure the builder to output a typed envelope `{system, static, dynamic, meta}` instead of a single string.
   - Add per-section budgets and 0.8-threshold compression triggers.
   - Persist compression artifacts and tool-output references for later retrieval.

2. **Summarizer & Citations**
   - Implement structured summarization templates for dynamic history and tool logs; keep citations to full logs.
   - Ensure meta memory is sourced from vetted `meta.recommendations` with timestamps and provenance.

3. **Meta-Steward Job**
   - Schedule a batch that replays long transcripts, harvests durable insights, and rewrites static/meta slices.
   - Apply schema validation to guard against malformed personas or policy drift.

4. **Observability & Tests**
   - Add gauges/counters for tokens, compression frequency, meta hit-rate, and cache miss detections.
   - Add long-horizon evals (50+ turns) validating goal continuity, compression triggers, and recovery from cache misses.

## Expected Benefits
- **Predictable latency & cost:** token budgets with early compression keep LLM calls stable across long sessions.
- **Higher signal retention:** structured summaries plus citations preserve decision-critical details while trimming noise.
- **Safer memory evolution:** meta-steward governance avoids stale/contradictory personas and enforces provenance.
- **Better observability:** metrics and compression reports make context behavior debuggable and auditable.

## TODO Roadmap (execute in order)
- [x] **Context builder refactor** *(budgets + artifacts landed; backfill CLI still pending)*
  - Deliverables: typed envelope interface in code, per-section budgets + 0.8 compression trigger, compression artifacts persisted with references to tool outputs, and a backfill script that snapshots current configs.
  - Acceptance: budget overrun triggers compression in dev; a 20+ turn trace shows the same persona/policies survive without cache churn.
  - Expected benefit: predictable latency (p95 within ±10%) and improved cache hit-rate because envelopes change only when sections change.
  - Implementation plan:
    1) Add a `context/envelope.go` (or equivalent) struct `{System, Static, Dynamic, Meta, Version, Metrics}` and update the builder entrypoints to emit this type instead of concatenated strings. **(done)**
    2) Introduce per-section budgets in configuration (`context.budget.system`, `context.budget.dynamic`, etc.) with a common `shouldCompress(section)` helper that triggers at 0.8. **(done; configurable budgets + threshold merged into BuildWindow)**
    3) Wire a compression registry that selects the appropriate summarizer per section and emits `{section, pre_tokens, post_tokens, kept_refs}` artifacts into a durable store (S3/file/db) with stable identifiers. **(done; artifacts persisted via pluggable store)**
    4) Build a backfill CLI (`context backfill --snapshot`) that serializes current personas/policies/meta to the new envelope format and stores them with versioned keys. **(todo)**
    5) Add regression fixtures with 20+ synthetic turns exercising budget overflow and verifying persona/policy stability across recompositions. **(todo; basic budget overflow fixture added)**

- [x] **Summarizer with citations** *(template-driven bullets + persisted citations; tool log persistence delegated to artifacts store)*
  - Deliverables: structured summarization templates (dynamic history, tool logs) that emit bullets + citations; last raw turn left uncompressed for verbatim recall; meta memory sourced only from vetted `meta.recommendations`.
  - Acceptance: eval prompt “你的上一句是什么” returns the exact last raw turn; summaries include at least one citation per bullet in test fixtures.
  - Expected benefit: higher factuality under compression and resilience to user quote-bait prompts.
  - Implementation plan:
    1) Define summarization templates in code (e.g., `templates/summarize_history.md`) with slots for citations and explicit guardrails (“do not alter quotes; cite source”).
    2) Implement a summarizer module (`summarizer/history.go`, `summarizer/tools.go`) that outputs structured JSON `{bullets: [], citations: []}` plus a `last_raw_turn` passthrough.
    3) Persist tool logs separately and inject only `kept_refs` into the envelope; add a lookup helper that resolves citations to full logs on demand.
    4) Add evaluators that issue “你的上一句是什么” and “引用第 3 条工具输出” to confirm verbatim recall and citation integrity on compressed contexts.
    5) Extend fixtures to require at least one citation per bullet and to fail if the last raw turn is not present verbatim.

- [x] **Meta-steward batch job** *(CLI replays journals into persona-scoped meta YAML/JSON with schema validation)*
  - Deliverables: scheduled job that replays long transcripts, extracts durable insights, rewrites static/meta slices atomically, and validates schema/provenance.
  - Acceptance: weekly run produces a changelog (who/when/why) with zero schema violations; drift checks prove personas stay consistent across runs.
  - Expected benefit: safer memory evolution and reduced contradictory persona/policy states.
  - Implementation plan:
    1) Add a replay pipeline (`cmd/meta-steward`) that ingests transcripts, runs a summarizer tuned for durable insights, and outputs candidate meta recommendations with provenance.
    2) Validate outputs against a JSON schema (`schemas/meta_recommendation.json`) and reject/flag any malformed or untrusted entries.
    3) Apply updates atomically to the static/meta sections in the envelope store, bumping a version and recording a changelog entry with author/time/reason.
    4) Add drift detection that diffs successive personas/policies and alerts when contradictions or large deltas are observed beyond a configured threshold.
    5) Schedule the job (cron/k8s) and export run metrics (`meta_steward.run.count`, `schema_violation.count`, `drift.detected`).

- [x] **Observability + long-horizon evals** *(per-section tokens/compression, meta hit-rate + cache-miss metrics, 40+ turn compression fixture)*
  - Deliverables: gauges/counters for tokens, compression frequency, meta hit-rate, cache-miss detections, plus 50+ turn eval suites covering goal continuity, compression triggers, and cache-miss recovery.
  - Acceptance: dashboards show per-section tokens and compression counts; long-horizon evals are green with <5% goal-drift failures.
  - Expected benefit: debuggable context behavior and measurable protection against cache breaks or injection regressions.
  - Implementation plan:
    1) Instrument context builds with metrics (`context.tokens_by_section`, `context.compress.count`, `context.cache_miss`) and expose them via Prometheus/OpenTelemetry.
    2) Add logs or traces that capture the envelope hash per request to correlate cache hits/misses and surface unexpected churn.
    3) Create long-horizon evaluation scenarios (≥50 turns) that deliberately trigger budget overflow, cache invalidation, and injection attempts; assert recovery and goal continuity.
    4) Integrate the evals into CI with thresholds (<5% goal drift, <1% unhandled cache misses); gate releases on green runs.
    5) Build dashboards combining metrics and eval results to visualize compression behavior, meta hit-rate, and attack-surface resilience over time.

## Cache Integrity & Attack Surface Analysis
- **Context cache compatibility:** segmented envelopes with per-section versioning reduce cache churn; compression artifacts are cached separately to prevent accidental invalidation.
- **User attacks ("你的上一句是什么" / injection):**
  - Do not rely on compressed summaries for verbatim recall; keep the last raw turn uncompressed for literal quotes.
  - Reject instructions that try to rewrite budgets/sections; schema validation and policy filters guard the builder inputs.
  - Auditors compare the loaded envelope against expected schema to detect missing sections or injected content.
- **Fallback behavior:** on cache misses, regenerate from persisted envelope snapshots and recompute summaries to avoid drifting responses.
