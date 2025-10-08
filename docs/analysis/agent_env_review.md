# Agent Architecture & Environment Configuration Review

## Summary
- Documented the current coordination flow built around `AgentCoordinator`, preparation services, and execution pipeline.
- Identified configuration pain points caused by hard-coded defaults and duplicated environment variable resolution logic across binaries and evaluation tooling.
- Proposed a phased remediation plan with concrete acceptance criteria covering refactors, observability, and regression protections.

## Phase 1 & 2 Implementation Status
- Introduced `internal/config` with a shared `RuntimeConfig` loader that merges defaults, config files, environment variables, and explicit overrides while tracking provenance metadata. CLI wiring, DI setup, evaluation harnesses, and built-in tools now depend on the unified loader instead of `os.Getenv`.
- Added preset observability by logging the source (config vs. context) and resulting tool counts whenever agent or tool presets are applied during execution preparation.
- Hardened MCP registry startup with exponential backoff retries, initialization status tracking, and an exported `MCPInitializationStatus` snapshot for health diagnostics.

## Phase 3 Implementation Status
- Refactored the SWE-Bench integration to construct agents through the shared DI container, applying runtime configuration overrides for model, sampling, and storage paths so batch runs share precedence rules with the CLI.
- Added configuration-focused integration tests that validate environment alias resolution, evaluation-specific storage directories, OpenAI base URL heuristics, and propagation of resolved settings into the coordinator.

## Phase 4 Implementation Status
- Added a repository-wide guard test that fails CI when new `os.Getenv` usages appear outside the sanctioned call sites, enforcing the shift to the shared runtime loader.【F:internal/config/env_usage_guard_test.go†L7-L71】
- Published an operations reference that consolidates runtime, evaluation, and ancillary environment variables, including defaults and alias coverage, so operators have a single source of truth.【F:docs/operations/runtime_environment.md†L1-L79】
- Routed the Tavily web search tool through the shared runtime config so API keys inherit the same precedence/alias handling as other credentials, and added unit tests that exercise both the configuration dependency and outbound request wiring.【F:internal/tools/registry.go†L23-L167】【F:internal/tools/builtin/web_search.go†L17-L209】【F:internal/tools/builtin/web_search_test.go†L1-L111】

## Phase 5 Implementation Status
- Eliminated the remaining CLI and server `os.Getenv` usage by parsing `ALEX_VERBOSE`, `ALEX_ENV`, and server port overrides through the shared runtime loader, threading the resolved values into streaming output, HTTP middleware, and the router while backfilling regression tests and documentation.【F:internal/config/loader.go†L21-L356】【F:cmd/alex/container.go†L1-L53】【F:cmd/alex/stream_output.go†L1-L214】【F:internal/server/http/middleware.go†L1-L64】【F:internal/server/http/middleware_test.go†L1-L60】【F:cmd/alex-server/main.go†L1-L178】【F:docs/operations/runtime_environment.md†L1-L74】

## Current Architecture Snapshot
The primary agent orchestration lives in `internal/agent/app`. The coordinator owns factories, registries, and persistence adapters, while also wiring together preparation, cost tracking, and analysis services during construction.【F:internal/agent/app/coordinator.go†L15-L99】 Runtime execution constructs a new `ReactEngine` per task and streams events, delegating session persistence post-run.【F:internal/agent/app/coordinator.go†L100-L199】 Execution preparation handles context compression, LLM client instantiation, preset resolution, and tool registry filtering before handing off to the core loop.【F:internal/agent/app/execution_preparation_service.go†L37-L148】【F:internal/agent/app/execution_preparation_service.go†L180-L320】 Dependency wiring happens inside the DI container, which also attempts to prime Git tooling with an LLM client and registers the sub-agent tool after coordinator creation.【F:internal/di/container.go†L20-L136】

## Environment Configuration Footprint
LLM and preset-related environment variables are resolved in multiple entry points:
- CLI configuration loader pulls API keys, provider/model overrides, and sampling parameters directly from the environment.【F:cmd/alex/config.go†L31-L145】
- The DI container exposes helpers for API key discovery and storage directories, again reading environment variables at runtime.【F:internal/di/container.go†L139-L204】
- Evaluation harnesses apply their own `ALEX_*` overrides for models, workers, and dataset selection separately from the runtime path.【F:evaluation/swe_bench/config.go†L180-L229】
This duplication increases drift risk because each surface silently falls back to different defaults.

## Key Issues
1. **Hidden Defaults Prevent Explicit Control** – `NewAgentCoordinator` overwrites a caller-supplied temperature of `0` with `0.7`, making it impossible to run deterministic (greedy) executions even if downstream config should allow it.【F:internal/agent/app/coordinator.go†L47-L132】
2. **Environment Resolution is Fragmented** – API keys, provider selection, and storage paths are loaded in three separate places (CLI, DI container, evaluation tooling) with subtly different precedence and validation rules.【F:cmd/alex/config.go†L31-L145】【F:internal/di/container.go†L139-L204】【F:evaluation/swe_bench/config.go†L180-L229】 This complicates debugging configuration problems and encourages copy-paste growth for new consumers (e.g., RAG CLI, batch runners).
3. **Preset Context is Implicit and Not Traceable** – Execution preparation reads preset overrides from `context.Context` without instrumentation, so downstream services cannot tell which persona/tool scope was active or why presets were chosen.【F:internal/agent/app/execution_preparation_service.go†L180-L320】 Lacking structured logs/metrics makes it difficult to audit or support preset-related incidents.
4. **Async MCP Registration Has No Backoff or Health Signal** – The DI container spawns registry initialization in a goroutine without retries or status exposure, so transient failures silently disable MCP tooling for the entire process.【F:internal/di/container.go†L90-L136】

## Optimization Plan
### Phase 1 – Configuration Unification
- Introduce a dedicated `internal/config` package that loads a strongly typed struct from environment variables, CLI flags, and optional files once, sharing it across CLI, DI, and evaluation code paths. Provide explicit precedence rules (CLI > env > file > defaults).【F:cmd/alex/config.go†L31-L145】【F:internal/di/container.go†L139-L204】【F:evaluation/swe_bench/config.go†L180-L229】
- Deprecate direct `os.Getenv` calls in downstream packages in favor of the shared loader. Add lint/test guardrails that fail if new `os.Getenv` usages appear outside the config layer.
- Update documentation to centralize environment variable references and default values in one table.

### Phase 2 – Agent Coordinator Controls & Telemetry
- Change temperature/TopP defaults to respect explicit zero values by switching to pointer semantics in `agentApp.Config` or introducing a sentinel (e.g., `nil` meaning "use default"). Add unit tests verifying that `0` is honored.【F:internal/agent/app/coordinator.go†L47-L132】
- Emit structured logs (or metrics events) when presets are selected, capturing source (config vs. context), preset name, and filtered tool counts. This addresses the observability gap noted in Issue 3.【F:internal/agent/app/execution_preparation_service.go†L180-L320】
- Expose MCP registry initialization status via the coordinator or DI container, with retry/backoff on failure and optional health checks to surface degraded tool availability.【F:internal/di/container.go†L90-L136】

### Phase 3 – Tooling & Evaluation Alignment
- Refactor evaluation harnesses to consume the shared configuration loader while allowing sandbox-specific overrides (e.g., dataset paths). Provide adapters or default constructors for batch scenarios so tests remain deterministic.【F:evaluation/swe_bench/config.go†L180-L229】
- Add smoke tests that spin up the container with mock providers to validate config propagation end-to-end (CLI → DI → coordinator → prep service).
- Document migration steps for other binaries (`rag_cli`, future services) and enforce via codeowners/review checklist.

## Acceptance & Validation Plan
1. **Config Loader Integration Tests** – Build table-driven tests that feed combinations of CLI flags, env vars, and JSON config to ensure precedence and defaulting logic match the specification. Failing cases (invalid numbers, missing keys) should produce actionable errors.
2. **Coordinator Behavior Tests** – Extend `internal/agent/app` tests to assert that explicit zero temperatures/top-p values persist through execution defaults, and that preset selection logs/metrics emit expected fields when context overrides are supplied.
3. **MCP Registry Reliability Checks** – Implement retryable mocks to simulate transient initialization failures, verifying that retries occur and status is surfaced via a health endpoint or log metric. Add acceptance criteria: retries with exponential backoff, capped attempts, and success event emission.
4. **End-to-End Smoke Scenario** – Run a scripted task invocation (mock LLM) exercising the unified configuration loader, ensuring environment overrides flow into the coordinator and tool registry (including preset filtering) without additional env lookups during execution.
5. **Documentation Sign-off** – Update ops/docs with the consolidated environment variable table and obtain review from DevOps/Infra stakeholders; acceptance requires signed checklist confirming deprecation of scattered env usage. _Status: awaiting stakeholder sign-off; documentation now covers Tavily API precedence and aliasing to match the codebase._【F:docs/operations/runtime_environment.md†L1-L79】
