# Context Engineering Improvement & Acceptance Plan

## 1. Background and Key Insights
- Anthropic's article *Effective context engineering for AI agents* frames context as "the minimal high-signal token set that maximizes the chance of desired behavior," emphasizing careful curation across prompts, tools, examples, and history (docs/research/context_engineering_article.md:1).
- The article warns about context rot in long sessions and recommends compaction, structured note-taking, and sub-agent orchestration to preserve alignment over extended horizons (docs/research/context_engineering_article.md:83).
- It promotes just-in-time retrieval combined with lightweight indices so agents hydrate precise artifacts only when needed, rather than pre-loading bulky material (docs/research/context_engineering_article.md:52).

## 2. Current Repository Assessment
- **Context management**: `internal/context/manager.go:6` relies on character-count heuristics and a fixed "system + last 10" retention rule; no semantic scoring, tool-output pruning, or multi-layer memory exists.
- **Prompt infrastructure**: despite new presets (`internal/agent/presets/prompts.go:1`), prompts remain monolithic strings per persona; there is no modular layering, capability-aware loading, or linting.
- **Session persistence**: the filestore (`internal/session/filestore/store.go:14`) stores raw message arrays only; there is no structured TODO/notes timeline beyond existing TODO tool files, and metadata lacks summaries or checkpoints.
- **Tool outputs & server streaming**: built-ins such as the subagent (`internal/tools/builtin/subagent.go:1`) and SSE broadcaster (`internal/server/app/event_broadcaster.go:1`) forward verbose payloads without automatic truncation, token accounting, or "high signal" annotations.
- **Observability & acceptance**: recent acceptance scripts under `tests/acceptance/` exercise APIs but do not track token usage, compression triggers, or memory effectiveness, leaving context engineering benefits unmeasured.

## 3. Improvement Roadmap
### 3.1 Prompt Governance & Modularity
- **Goal**: decompose persona prompts into reusable segments that can be composed per model capability, task type, and tool preset.
- **Actions**:
  1. Introduce a declarative prompt manifest format (`internal/agent/presets/`) combining identity, workflow, tool guidance, and output directives.
  2. Extend `prompts.Loader` to assemble prompts via feature flags (eg, minimal core, security overlay, TODO workflow) and expose metrics on resulting token footprint.
  3. Add a lint script (`scripts/`) that checks for duplicated guidance, missing section headers, and maximum token budgets per preset.

### 3.2 Layered Context & Compression Pipeline
- **Goal**: evolve the context manager into a pipeline that separates short-term turn context, persistent summaries, and tool caches while respecting token budgets.
- **Actions**:
  1. Define a `Summarizer` interface and default implementation (LLM-backed or heuristic) under `internal/context/` to consolidate stale history before reaching hard limits.
  2. Expand `ports.ContextManager` with `PrepareForLLM` that returns curated messages plus auxiliary sections (eg, `notes`, `summary`, `recent_tools`).
  3. Integrate tool-output sanitizers so commands like `file_read` or `subagent` register references and deltas instead of raw blobs.

### 3.3 Structured Notes & Long-Horizon Memory
- **Goal**: support planned compaction cycles and note-taking recommended in the article.
- **Actions**:
  1. Extend `ports.Session` to add `Notes`, `Artifacts`, and `Checkpoints` collections persisted by the filestore.
  2. Ship dedicated tools (`notes_write`, `notes_read`) or extend TODO tooling to manage structured progress logs tied to milestones.
  3. During compression, snapshot outstanding tasks and inject concise summaries into notes before truncating message history.

### 3.4 Just-in-Time Retrieval & Tool Signals
- **Goal**: give the agent cost-aware cues for runtime retrieval and avoid bloated context.
- **Actions**:
  1. Augment `ports.ToolMetadata` with `SignalStrength`, `TokenCostHint`, and `SupportsPreview` so planners can choose token-efficient paths.
  2. Add a retrieval coordinator in `internal/agent/domain/` that orchestrates `list_files`, `ripgrep`, and `file_read` into progressive disclosure passes.
  3. Require subagents to return capped, reference-heavy summaries (enforced in `internal/tools/builtin/subagent.go`) and optionally store raw transcripts as off-context artifacts.

### 3.5 Observability & Acceptance Expansion
- **Goal**: ensure the new context strategies are measurable and regression-tested.
- **Actions**:
  1. Enhance `ports.CostTracker` and SSE instrumentation to log per-turn token counts, compression invocations, and summarizer latencies.
  2. Extend `tests/acceptance/run_all_tests.sh` with long-horizon journeys (50+ turns) asserting continuity of goals, token caps, and note usage.
  3. Provide dashboards or reports under `logs/` summarizing context efficiency deltas compared with the baseline.

## 4. Phased Deliverables & Acceptance Criteria
| Phase | Duration | Focus | Key Deliverables | Acceptance Signals |
| --- | --- | --- | --- | --- |
| 0 | 1 week | Baseline & design | Current context footprint report, prompt audit, target KPIs | Design review approved; `go test ./...` stays green |
| 1 | 2 weeks | Prompt modularity + linting | Prompt manifests, loader refactor, lint script | Preset prompts shrink ≥25% tokens; lint CI job passing |
| 2 | 3 weeks | Layered context pipeline | New `ContextManager`, summarizer, tool-output hooks | Multi-turn integration test continues after compression with intact goals |
| 3 | 3 weeks | Notes & JIT retrieval | Session schema update, notes tools, retrieval coordinator | Acceptance scenario shows ≥30% reduction in avg input tokens on large repo task |
| 4 | 2 weeks | Observability & regression | Metrics logging, dashboards, long-horizon acceptance suite | Long-run test (>50 turns) maintains task continuity; metrics exported to logs |

Shared acceptance principles:
- **Correctness**: New APIs include unit and integration tests; CI remains stable.
- **Efficiency**: Token usage trends downward or remains bounded; tool outputs respect 1k-token default caps.
- **Rollback**: Feature flags guard major components (eg, `context.layered_enabled`).
- **Documentation**: Update `docs/reference/` and `docs/guides/` with operational guidance for the new context pipeline.

## 5. Risks & Follow-up
- Summarizer quality depends on available models; plan rule-based fallbacks for air-gapped or offline deployments.
- Additional telemetry may introduce overhead; benchmark SSE and task APIs to ensure latency remains acceptable.
- Coordinate with product and operations stakeholders at each phase boundary to realign priorities and capture feedback.
