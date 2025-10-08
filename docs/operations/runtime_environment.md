# Runtime Environment Reference

This reference centralises the environment variables that influence the agent runtime, batch evaluation harness, and auxiliary tooling.

## Core runtime configuration

| Variable | Aliases | Default | Description |
| --- | --- | --- | --- |
| `OPENAI_API_KEY` | `OPENROUTER_API_KEY` | _none_ | Primary API key used for non-mock LLM providers. The shared loader falls back to the mock provider when no key is present.【F:internal/config/loader.go†L205-L246】 |
| `LLM_PROVIDER` | `ALEX_LLM_PROVIDER` | `openrouter` | Selects which LLM integration to use at runtime.【F:internal/config/loader.go†L178-L207】【F:cmd/alex-server/main.go†L118-L132】 |
| `LLM_MODEL` | `ALEX_LLM_MODEL`, `ALEX_MODEL_NAME` | `deepseek/deepseek-chat` | Default chat completion model identifier consumed by all binaries and SWE-Bench integration runs.【F:internal/config/loader.go†L178-L241】【F:evaluation/swe_bench/env_aliases.go†L4-L12】 |
| `LLM_BASE_URL` | `ALEX_BASE_URL` | `https://openrouter.ai/api/v1` | Base URL for HTTP calls to the LLM provider. Useful when targeting self-hosted gateways.【F:internal/config/loader.go†L178-L241】【F:cmd/alex-server/main.go†L118-L132】 |
| `LLM_MAX_ITERATIONS` | `ALEX_LLM_MAX_ITERATIONS` | `150` | Caps the number of agent dialogue turns before aborting execution.【F:internal/config/loader.go†L178-L241】【F:cmd/alex-server/main.go†L118-L132】 |
| `LLM_MAX_TOKENS` | `ALEX_LLM_MAX_TOKENS`, `ALEX_MODEL_MAX_TOKENS` | `100000` | Upper bound on response tokens requested from the provider.【F:internal/config/loader.go†L178-L241】【F:evaluation/swe_bench/env_aliases.go†L4-L12】 |
| `LLM_TEMPERATURE` | `ALEX_MODEL_TEMPERATURE` | `0.7` | Controls sampling randomness. Explicit zero values are preserved and propagated across the runtime stack.【F:internal/config/loader.go†L178-L246】【F:internal/agent/app/coordinator.go†L47-L132】 |
| `LLM_TOP_P` | – | `1.0` | Alternative nucleus sampling parameter applied to the shared completion defaults.【F:internal/config/loader.go†L178-L246】 |
| `LLM_STOP` | – | _empty_ | Optional comma/space-delimited list of stop sequences injected into completion requests.【F:internal/config/loader.go†L232-L246】 |
| `TAVILY_API_KEY` | `ALEX_TAVILY_API_KEY` | _none_ | API key used by the built-in Tavily web search tool. Resolved via the shared runtime loader so CLI, server, and SWE-Bench runs share precedence and alias handling.【F:internal/config/loader.go†L205-L356】【F:internal/tools/registry.go†L23-L167】 |
| `ALEX_SESSION_DIR` | – | `~/.alex-sessions` | Filesystem location for saving execution transcripts and artefacts.【F:internal/config/loader.go†L178-L241】 |
| `ALEX_COST_DIR` | – | `~/.alex-costs` | Directory for persisted cost-tracking data emitted by the coordinator.【F:internal/config/loader.go†L178-L241】 |

## Batch evaluation overrides

| Variable | Aliases | Default | Description |
| --- | --- | --- | --- |
| `NUM_WORKERS` | `ALEX_NUM_WORKERS` | `1` | Overrides the number of parallel SWE-Bench workers spawned during batch runs.【F:evaluation/swe_bench/config.go†L206-L224】【F:evaluation/swe_bench/env_aliases.go†L4-L12】 |
| `OUTPUT_PATH` | `ALEX_OUTPUT_PATH` | `./batch_results` | Specifies where batch run outputs and logs are written.【F:evaluation/swe_bench/config.go†L214-L224】【F:evaluation/swe_bench/env_aliases.go†L4-L12】 |
| `DATASET_TYPE` | `ALEX_DATASET_TYPE` | `swe_bench` | Controls which dataset implementation the evaluation harness targets.【F:evaluation/swe_bench/config.go†L220-L228】【F:evaluation/swe_bench/env_aliases.go†L4-L12】 |
| `DATASET_SUBSET` | `ALEX_DATASET_SUBSET` | `lite` | Selects the SWE-Bench subset (lite, full, verified).【F:evaluation/swe_bench/config.go†L220-L228】【F:evaluation/swe_bench/env_aliases.go†L4-L12】 |
| `DATASET_SPLIT` | `ALEX_DATASET_SPLIT` | `dev` | Chooses which split to evaluate (dev, test, train).【F:evaluation/swe_bench/config.go†L220-L228】【F:evaluation/swe_bench/env_aliases.go†L4-L12】 |

## Ancillary tooling

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listen port for the SSE server binary, resolved through the shared alias-aware environment lookup before constructing the HTTP server.【F:cmd/alex-server/main.go†L133-L177】 |
| `ALEX_ENV` | `development` | Determines whether the HTTP middleware treats incoming origins as trusted (development) or requires explicit allow-list entries (production). Resolved by the shared loader and threaded into the CORS middleware.【F:internal/config/loader.go†L205-L356】【F:internal/server/http/middleware.go†L10-L64】 |
| `ALEX_VERBOSE` | `false` | Enables verbose streaming output from the CLI, mirroring the `--verbose` flag when set to truthy values. Parsed by the runtime loader and supplied through the CLI container to streaming renderers.【F:internal/config/loader.go†L205-L356】【F:cmd/alex/container.go†L11-L49】【F:cmd/alex/stream_output.go†L23-L120】 |

> **Note:** The MCP registry loader expands environment variables found within MCP configuration files at runtime. Those references are not enumerated here because they are dynamic, but the expansion pipeline emits warnings for missing variables to aid debugging.【F:internal/mcp/config.go†L204-L248】 Runtime configuration values resolved by `internal/config` are surfaced to the loader as synthetic environment entries, so placeholders such as `${OPENAI_API_KEY}` inherit the same precedence rules as the rest of the runtime.【F:internal/config/runtime_env.go†L1-L78】【F:internal/di/container.go†L1-L208】【F:internal/mcp/registry.go†L1-L205】
