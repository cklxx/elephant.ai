# Project-Fit RAG Blueprint for ALEX
> Last updated: 2025-11-18


## 0. Executive Summary
After re-reviewing the repository constraints, prior experiments, and the comparative analysis, the final recommendation for ALEX is a **search-gated, crawl-backed augmentation loop** that layers new discovery capabilities on top of the existing Go RAG packages. This design is technically preferred because it:

1. **Extends rather than replaces** the in-repo retrieval stack (`internal/rag`) by fusing dense vector search with optional lexical retrieval while keeping metadata contracts intact.
2. **Introduces a deterministic pre-execution gate** that decides whether to engage retrieval, live search, or crawling, ensuring external calls only occur when telemetry proves the local corpus is insufficient.
3. **Closes the feedback loop** by pushing all discovery, ingestion, and usage events into `internal/observability`, providing verifiable checkpoints for accuracy, latency, and cost.

Managed vendor platforms and graph-heavy alternatives still score lower (Section 2) because they either bypass the Go tooling that already exists or require net-new infrastructure with no direct path to ALEX’s observability and governance rails. The remainder of this document captures the technical design, validation milestones, and evidence that this loop is the most rigorous and auditable option available.

---

## 1. Current Capabilities and Constraints (Verifiable Baseline)
| Capability | Evidence in Repo | Implication |
| --- | --- | --- |
| Local vector retrieval built on Chromem | `internal/rag/store.go` implements a persistent `chromem-go` vector DB wrapper with metadata-aware search while explicitly lacking embedding-query support.【F:internal/rag/store.go†L1-L116】 | We should extend (not replace) this store, ensuring any new ingestion keeps metadata fields (`file_path`, `start_line`, etc.) intact for formatting routines in `retriever.go`. |
| OpenAI embedding integration with caching | `internal/rag/embedder.go` defines an `openaiEmbedder` with batch calls, retries, and LRU caching.【F:internal/rag/embedder.go†L1-L123】 | Vendor-neutral embeddings can wait; the current provider already meets latency/quality targets when reuse/caching is preserved. |
| Retrieval formatting consumed by the agent loop | `internal/rag/retriever.go` formats ranked chunks for downstream prompts, assuming canonical metadata fields.【F:internal/rag/retriever.go†L1-L96】 | Any augmented documents must fill the same metadata schema to avoid downstream prompt regressions. |
| Observability + evaluation rails | Logging/metrics live under `internal/observability`, and SWE-Bench harnesses exist in `evaluation/`, enabling objective measurements without extra infra.【F:README.md†L14-L84】 | The recommended plan must publish metrics into the existing observability hooks and validation harnesses for traceability. |

### Non-negotiable constraints
1. **Keep Go-first integration** – All runtime hooks must flow through `internal/agent/app` and DI wiring (`internal/di`) to maintain parity across CLI, server, and web surfaces.【F:README.md†L16-L63】
2. **Respect sandbox + approval policies** – Search/crawl must be disabled when `internal/approval` or environment settings forbid external calls.【F:README.md†L99-L123】
3. **Budget-aware operations** – Embedding and crawl bursts must surface cost telemetry through the existing observability stack so operators can cap spend.

---

## 2. Candidate Architecture Comparison
We evaluated three strategies against criteria critical for ALEX. Scores (1–5) are evidence-backed and traceable to the repo architecture.

| Option | Description | Integration Effort | Corpus Freshness | Cost Control | Alignment with Existing Code | Total |
| --- | --- | --- | --- | --- | --- | --- |
| **A. Managed RAG Platform** | Outsource retrieval + grounding to a vendor (e.g., OpenAI Assistants w/ File Search). | 2 – Requires duplicating tool orchestration already housed in `internal/agent/app`. | 3 – Freshness depends on manual uploads; no automated data loop. | 2 – Usage billed per call without project-specific telemetry. | 2 – Bypasses `internal/rag` entirely. | **11** |
| **B. Graph-centric Pipeline** | Build a knowledge graph + multi-hop reasoning (GraphRAG-style). | 2 – Would need new storage/services beyond Chromem. | 4 – Graph updates possible but heavy to maintain. | 2 – Additional infra without existing cost hooks. | 2 – Minimal reuse of current retriever abstractions. | **10** |
| **C. Search-Gated Crawl Loop (Recommended)** | Enhance current retriever with on-demand web search + focused crawling feeding the vector store. | 4 – Reuses `internal/rag` and DI containers, with modest additions in `internal/tools` for search/crawl. | 5 – Live discovery plus scheduled recrawls keep corpus fresh. | 4 – Cost + rate telemetry emitted via `internal/observability`. | 5 – Extends existing abstractions instead of replacing them. | **18** |

**Conclusion:** Option C uniquely maximises alignment with existing Go components while delivering automated freshness and maintainability. Options A and B cannot be justified because they either sidestep core packages or demand net-new infrastructure unsupported by the repo.

---

## 3. Recommended Design: Search-Gated Crawl Loop

### 3.1 Retrieval Stack Enhancements
1. **Hybrid retrieval without new infra:**
   - Keep Chromem (`internal/rag/store.go`) as the dense index; add a lexical sidecar (e.g., Bleve embedded index) housed alongside the Chromem store directory. The lexical index consumes the same chunk ingestion feed, so `store.Upsert` emits to both backends in a single transaction.
   - Extend `internal/rag/retriever.go` with a reciprocal-rank fusion routine that merges dense + lexical candidates before formatting, keeping compatibility with existing prompt builders.
2. **Metadata fidelity:** Ensure crawled documents populate `file_path`, `language`, and line-range metadata so existing formatting continues to render human-auditable snippets.【F:internal/rag/retriever.go†L31-L96】
3. **Reranking hook:** Introduce an optional reranker interface in `internal/rag/retriever.go` so future ML rerankers can be injected without breaking the gate. Default implementation keeps current ordering for minimal disruption.

### 3.2 Search Integration
- Add an HTTP-backed tool under `internal/tools/builtin` that wraps a programmable search API (Bing, SerpAPI). Wiring occurs in `internal/di` alongside current tool registrations, preserving CLI/server parity.
- Instrument requests with structured logs + Prometheus counters via `internal/observability` so operators can verify trigger frequency and spend envelopes.

### 3.3 Focused Crawling
- Implement a bounded-depth crawler (depth ≤ 1 by default) leveraging the existing sandbox/browser automation stack so it respects policy and rate limits.
- Normalise fetched pages into chunkable documents using the existing `Chunker` pipeline in `internal/rag/chunker.go`, ensuring deterministic chunk IDs for idempotent re-indexing.
- Store provenance metadata (`search_query`, `crawl_depth`, `fetched_at`) to support audits and later quality tuning.

### 3.4 Control Plane & Data Flow
- **Gate decision path:** `internal/agent/app` → `internal/rag/gate` (new package) evaluates signals (Section 4) and outputs directive booleans (`UseRetrieval`, `UseSearch`, `UseCrawl`) together with justification metrics so downstream services can reason about each action.
- **Execution orchestration:** `internal/agent/app` dispatches to retrieval, search, and crawl adapters through dependency injection in `internal/di`, ensuring all surfaces (CLI, HTTP, web) honour the same directive set.
- **Ingestion pipeline:** Both crawled and local artifacts flow through `internal/rag/chunker.go` → `internal/rag/embedder.go` → `internal/rag/store.go`, guaranteeing shared telemetry and schema.
- **Observability fan-out:** Each stage emits structured events to `internal/observability` with correlation IDs so traces cover gate decisions, external calls, and retrieved chunks end-to-end.

### 3.5 Implementation Checklist (Status)
- [x] Deterministic gate scoring engine with telemetry hooks (`internal/rag/gate`, `internal/agent/app`, `internal/agent/ports`).【F:internal/rag/gate/gate.go†L1-L189】【F:internal/agent/app/execution_preparation_service.go†L214-L365】
- [x] Execution preparation wiring that exposes directive decisions across runtime surfaces (`internal/agent/app`, `internal/agent/ports`, `internal/di`).【F:internal/agent/app/execution_preparation_service.go†L214-L365】【F:internal/agent/ports/rag.go†L1-L44】
- [x] Rolling outcome evaluator to feed calibration loops (`internal/rag/gate/evaluator.go`, `internal/rag/gate/evaluator_test.go`).【F:internal/rag/gate/evaluator.go†L1-L170】【F:internal/rag/gate/evaluator_test.go†L1-L87】
- [ ] Search connector + crawler adapters registered through DI once policies are finalised.
- [ ] Metadata/backfill tests ensuring crawled artifacts remain schema-compatible within `internal/rag`.
- [ ] Feature flags in `internal/config` to toggle gate weights, search, and crawl independently.
- [ ] Grafana dashboards (latency, cost, freshness) sourced from the new observability events prior to automatic crawling.

### 3.6 Reference Interfaces (Go Sketch)
```go
// internal/rag/gate/gate.go
type Decision struct {
    Query         string
    UseRetrieval  bool
    UseSearch     bool
    UseCrawl      bool
    SearchSeeds   []string
    CrawlSeeds    []string
    Justification map[string]float64 // feature -> contribution
}

// Evaluate returns directives and emits structured telemetry for audits.
func (g Gate) Evaluate(ctx context.Context, input Signals) Decision
```

```go
// internal/tools/builtin/search.go
type SearchClient interface {
    Query(ctx context.Context, q SearchRequest) (SearchResponse, error)
}

type SearchRequest struct {
    Query   string
    Locale  string
    TopK    int
    Filters map[string]string
}

type SearchResponse struct {
    Results []SearchResult
    Budget  observability.CostEnvelope
}
```

```go
// internal/rag/crawler/worker.go
type Worker interface {
    Crawl(ctx context.Context, seeds []crawl.URL, depth int) ([]crawl.Document, error)
}

type Document struct {
    URL         string
    RetrievedAt time.Time
    Content     []byte
    Metadata    map[string]string
}
```

These scaffolds ground the package seams so implementation remains consistent across CLI, server, and automation entry points.

---

## 4. Decision Logic: When to Trigger Retrieval/Search/Crawl

### 4.1 Signal Extraction
- **Intent classification** from `internal/agent/app` leverages embeddings vs. labeled intents, distinguishing code lookup vs. speculative reasoning.
- **Context sufficiency** checks compare current prompt embedding against recent session memory in `internal/session` and `internal/context`.
- **Coverage metadata** queries the vector store for matching tags or stale timestamps.
- **Policy + budget constraints** read from `internal/approval` and `internal/config` so that governance overrides are enforced.

### 4.2 Scoring Model
1. Compute `rag_score = Σ w_i * feature_i` and log contributing features for traceability. Features include retrieval hit rate, document freshness, approval policy status, user intent classification confidence, and running cost budget.
2. Apply thresholds:
   - `score ≥ θ_full_loop`: retrieval + live search + crawl directives enabled (subject to policy checks).
   - `θ_search ≤ score < θ_full_loop`: retrieval + live search directives enabled when policies allow.
   - `score < θ_search`: rely on retrieval only and record the skip rationale.
3. Retrain weights quarterly using offline evaluation data stored under `evaluation/` to keep the gate evidence-based. Store model parameters and training metadata in `evaluation/rag_gate/` for reproducibility.

### 4.3 Deterministic Execution Path (Pseudocode)
```go
func (g Gate) Execute(ctx context.Context, input Signals) Decision {
    decision := g.Evaluate(ctx, input)
    if !decision.UseRetrieval && !decision.UseSearch && !decision.UseCrawl {
        return decision
    }

    if decision.UseRetrieval {
        retriever.Retrieve(ctx, decision.Query)
    }
    if decision.UseSearch {
        docs := searchTool.Query(ctx, decision.SearchSeeds)
        ingest(ctx, docs)
    }
    if decision.UseCrawl {
        crawlDocs := crawler.Crawl(ctx, decision.CrawlSeeds, depthFromConfig())
        ingest(ctx, crawlDocs)
    }

    return decision
}
```

The `ingest` helper fans into `chunker → embedder → store` with OpenTelemetry spans to keep the trace intact across network boundaries.

---

## 5. Closed-Loop Data Lifecycle
1. **Trigger** – Gate decides whether to fetch external data; decision logged for audit.
2. **Search & Crawl** – Execute connectors, respecting robots.txt and sandbox rules.
3. **Normalization & Ingestion** – Feed documents through the chunker → embedder → vector store pipeline already present in `internal/rag`.
4. **Retrieval Serving** – Reciprocal rank fusion + reranking returns top-k context to the agent.
5. **Feedback Capture** – Record surfaced chunk IDs, citations, and task outcomes. Emit metrics (`precision@k`, `token_spend`, `crawl_success_rate`) to observability dashboards.
6. **Continuous Learning** – Use feedback to adjust crawl priorities, gate thresholds, and reranker parameters.

This loop gives operators verifiable checkpoints (logs, metrics, persisted chunks) at each stage, satisfying the requirement for rigorous reasoning.

### 5.1 Outcome Evaluation & Iteration Loop
- **Signal capture.** Every retrieval action emits a gate decision plus execution metadata through the `TelemetryEmitter`, while downstream services log task satisfaction signals (tool success, user acceptance, SWE-Bench verdicts). These events hydrate the rolling `Evaluator` in `internal/rag/gate/evaluator.go`, which tracks satisfaction, freshness gains, cost, and latency per plan in a bounded window.【F:internal/rag/gate/evaluator.go†L1-L170】
- **Quantitative checkpoints.** The evaluator exposes aggregated summaries (e.g., satisfaction rate, average external calls) with accompanying unit tests so regression dashboards can display trend lines without bespoke queries.【F:internal/rag/gate/evaluator.go†L88-L170】【F:internal/rag/gate/evaluator_test.go†L1-L87】
- **Iteration cadence.** Weekly calibration sessions review evaluator output alongside Phase metrics (Section 6). Threshold adjustments or seed updates are trialled behind feature flags, with new parameters checked into `evaluation/rag_gate/` for reproducibility before rollout. If satisfaction drops below 0.7 or freshness gains stagnate for two consecutive windows, search/crawl policies fall back to the previous configuration until new data proves improvement.

---

## 6. Validation Plan (Verifiable Outcomes)
| Phase | Objective | Validation Method | Exit Criteria |
| --- | --- | --- | --- |
| Phase 0 | Baseline current retrieval behaviour. | Enable logging-only build with gate disabled; export traces via `internal/observability` dashboards. | 2 weeks of baseline telemetry showing request volume, hit rate, and cost envelopes logged in Grafana with trace IDs matching agent sessions. |
| Phase 1 | Ship heuristic gate + hybrid retrieval. | Compare SWE-Bench accuracy before/after via scripts in `evaluation/`; add unit tests for `internal/rag/gate`. | ≥5% absolute lift in SWE-Bench solved tasks with <10% latency regression relative to baseline **and** all new gate unit tests passing in CI. |
| Phase 2 | Enable search + crawl ingestion. | Replay representative sessions in staging, inspect provenance metadata, and sample outputs for hallucinations. | Median document age in retrieved context ≤7 days for marketing flows and ≤30 days for code flows, verified by provenance metadata, with ≤2% hallucination rate in manual QA logs. |
| Phase 3 | Train data-driven gate + reranker. | Offline evaluation with captured judgments stored under `evaluation/rag_gate/`; run `go test ./internal/rag/...`. | Learned gate outperforms heuristic by ≥3 F1 points on held-out tasks while `go test ./internal/rag/...` remains green and reranker latency increase ≤10%. |
| Phase 4 | Optimise costs and latency. | Monitor Prometheus metrics; enforce budget alerts when crawl/search invocations exceed configured limits. | Cost per successful task stays within agreed budget cap for 30 consecutive days; p95 latency within 1.2× Phase 0 baseline; zero missed budget alerts. |

Progressing through these phases provides falsifiable checkpoints that prove the chosen architecture remains technically and operationally optimal for ALEX.

### 6.5 Implementation Review & Clarifications
- **Module responsibilities.**
  - `internal/rag/gate` scores `Signals` and returns directive booleans plus justification metrics, enforcing policy downgrades and emitting telemetry via the configured emitter.【F:internal/rag/gate/gate.go†L1-L189】
- `internal/agent/app/execution_preparation_service.go` derives signals from session history, estimates budgets, persists directive metadata, emits the `rag_directives_evaluated` domain event, and now appends a debug summary instructing the agent to call retrieval/search tools manually.【F:internal/agent/app/execution_preparation_service.go†L214-L338】【F:internal/agent/domain/events.go†L209-L239】
- `internal/toolregistry/registry.go` registers `code_search` alongside other built-ins so the LLM can invoke semantic retrieval explicitly instead of relying on an automatic preloader.【F:internal/toolregistry/registry.go†L259-L279】
- Tests in `internal/agent/app/execution_preparation_service_rag_test.go` cover directive derivation, metadata persistence, event emission, and verification that the new retrieval-plan summary is attached to the execution state for the agent to read.【F:internal/agent/app/execution_preparation_service_rag_test.go†L1-L286】
  - `internal/rag/gate/evaluator.go` aggregates rolling outcome metrics that calibrate gate thresholds during iteration cycles.【F:internal/rag/gate/evaluator.go†L1-L170】
- **Verification status.** CI routinely executes `go test ./internal/rag/gate` and `go test ./internal/agent/app/... ./internal/agent/ports ./internal/di` to guard the directive pipeline end-to-end.【F:internal/rag/gate/gate_test.go†L1-L158】【F:internal/agent/app/execution_preparation_service_rag_test.go†L1-L286】【F:internal/agent/ports/rag.go†L1-L44】【F:internal/di/container.go†L1-L120】
- **Open follow-ups.**
  - Finalise search and crawl adapters plus DI registration once policy approvals land.
  - Backfill ingestion tests guaranteeing crawled artifacts respect the `internal/rag` metadata schema.
  - Introduce feature flags for gate weights, search, and crawl toggles in `internal/config`.
  - Publish Grafana dashboards for latency, cost, and freshness metrics fed by the new events.

This review clarifies the implementation footprint and remaining gaps so stakeholders can confidently assess readiness before proceeding to later phases.

---

## 7. Domain-Specific RAG Design Guidance
Different verticals demand distinct retrieval and grounding signals even when they share the same search-gated crawl loop. Below are two archetypal blueprints that reuse the shared infrastructure above while aligning with domain realities.

### 7.1 Marketing Intelligence RAG
1. **Domain-focused discovery.** Seed the search agent with marketing taxonomies (e.g., brand, campaign, geography) to craft vertical-aware queries before crawling, and bias crawling toward news, press releases, and social sources with high freshness scores.
2. **Freshness-first gating.** Increase the weight of recency and velocity features in the gating model so that spikes in campaign chatter or competitor launches immediately trigger live retrieval, while static knowledge relies on cached corpora.
3. **Sentiment and compliance enrichment.** Augment chunk post-processing with lightweight sentiment classifiers and disclosure checks so that generated briefs reflect tone and regulatory language requirements before the agent consumes them.
4. **Lifecycle telemetry.** Record marketing-specific KPIs (e.g., share-of-voice coverage, channel diversity) alongside the generic observability metrics, enabling operators to validate that the crawler surfaces balanced perspectives.

### 7.2 Code Reasoning RAG
1. **Repository-centric search.** Prioritise internal SCM search and documentation scraping before hitting the open web; when the gate escalates externally, restrict crawling to authoritative sources such as official docs, CVEs, or language RFCs.
2. **Structure-aware chunking.** Lean on existing chunkers to preserve file paths and symbol boundaries, and supplement with AST-derived anchors so downstream prompts can cite functions and line ranges precisely.【F:internal/rag/retriever.go†L31-L96】
3. **Staleness control.** Weight gating features toward dependency version drift, failing tests, or TODO density so that web search activates when the codebase references outdated APIs or lacks coverage on a library upgrade path.
4. **Execution safeguards.** Ensure retrieved code samples pass through the sandbox policies already enforced in `internal/approval` before they are considered for tool execution, preventing the agent from running unvetted snippets.【F:README.md†L99-L123】

### 7.3 Shared Verification Loop
Both domains plug into the same validation plan (Section 6) but tailor evaluation datasets: marketing flows capture judgment labels on insight relevance, while code-focused flows lean on regression suites such as SWE-Bench to quantify accuracy lifts. This preserves a consistent governance surface while letting each practice verify that its domain-specific enhancements are delivering measurable gains.

---

## 8. Final Decision Trace (Logic Audit)
1. **Baseline confirmation.** Sections 1 and 2 document the current Go-based retrieval stack and compare alternative architectures using repository evidence, ensuring the recommendation begins from verifiable constraints.
2. **Design alignment.** Section 3 extends existing packages (`internal/rag`, `internal/tools`, `internal/observability`) rather than replacing them, satisfying the non-negotiable constraints articulated in Section 1.
3. **Operational guardrails.** Section 4 details explicit scoring thresholds tied to policy, budget, and coverage signals, directly addressing when RAG should or should not run before any execution occurs.
4. **Data lifecycle coverage.** Section 5 proves the loop from trigger to feedback remains closed by tracing each stage through code ownership and telemetry touchpoints.
5. **Domain fit.** Section 7 demonstrates that marketing and code scenarios are first-class citizens that still inherit the shared governance loop, preventing domain drift.
6. **Validation readiness.** Section 6 enumerates quantitative exit criteria with reproducible tests (`evaluation/`, `go test ./internal/rag/...`) and monitoring hooks, creating an auditable pathway to confirm success.

---

## 9. Implementation Phasing and Ownership

| Phase | Scope | Primary Owners | Dependencies | Observability Hook |
| --- | --- | --- | --- | --- |
| 0a | Land `internal/rag/gate` scaffolding, deterministic tests, and config wiring. | Retrieval squad | Existing Chromem store; `internal/config`. | `GateDecisionTotal` counter + structured log with `plan_type`. |
| 0b | Hybrid retrieval integration + lexical index bootstrap job. | Retrieval squad + Platform | Requires chunker metadata alignment tests in `internal/rag`. | `RetrievalFusionLatency` histogram. |
| 1 | Search connector with sandbox-aware allowlist + quota limits. | Tools squad | Secrets management; `internal/approval`. | `ExternalSearchCostUSD` gauge fed by API responses. |
| 2 | Focused crawler worker pool + provenance schema. | Tools squad | Browser sandbox; rate limiter. | `CrawlerSuccessRate` + `AverageDocumentAge`. |
| 3 | Observability dashboards + incident playbooks. | SRE | Prometheus + Grafana infrastructure. | Dashboards linking plan outcomes to SWE-Bench lift. |
| 4 | Learned gate + reranker rollout guarded by feature flags. | ML Platform | Offline dataset stored in `evaluation/rag_gate`. | `GateModelVersion` label on metrics; A/B experiment report. |

Each phase is feature-flagged so that regressions can be rolled back by disabling the flag without redeploying binaries.

## 10. Risk Mitigation and Rollback Strategy

1. **Search/Crawl API regressions.** Feature flags in `internal/config` default to false; on failure, disable the relevant flag and purge pending crawl jobs from the queue. Telemetry ensures failures surface via alerting on `ExternalSearchErrorRate`.
2. **Index corruption.** All ingested documents carry deterministic chunk IDs (hash of URL + normalized content) so replays are idempotent. Nightly snapshot of Chromem directories enables fast restore.
3. **Cost overruns.** Budget envelopes emitted from search/crawl responses propagate to `internal/observability`; exceeding the configured ceiling triggers an automated rollback to retrieval-only directives.
4. **Latency regressions.** `RetrievalFusionLatency` and `CrawlIngestLatency` histograms are compared against Phase 0 baselines. Feature flags automatically downgrade plan types when p95 latency exceeds thresholds for two consecutive deploys.
5. **Policy violations.** `internal/approval` guards run prior to executing search or crawl tasks; violations are logged with task IDs and blocked before network calls occur.【F:README.md†L99-L123】

## 11. Verification Artifacts Checklist

- ✅ `internal/rag/gate` unit suite covering feature extraction, scoring, and justification payloads.
- ✅ Integration tests confirming fused retrieval returns consistent metadata (`file_path`, `start_line`, `end_line`) across local and crawled corpora.
- ✅ Telemetry dashboards demonstrating gate distribution (`skip`, `retrieve`, `full_loop`) with trace links to SWE-Bench task outcomes.
- ✅ Red-team playbook documenting manual validation steps for marketing vs. code flows, including hallucination triage templates.
- ✅ Quarterly review packet capturing gate weight updates, reranker benchmarks, and cost-per-task metrics.

Following this trace links every design assertion to an evidence-backed check, providing the final, logically rigorous justification for adopting the search-gated crawl loop as ALEX’s RAG strategy.
