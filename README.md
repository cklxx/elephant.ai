# Spinner

Spinner is an AI agent that turns scattered facts, logs, and scratch notes into an actionable knowledge web. It runs the same layered Go backend that powers ALEX, and is focused on weaving fragmented context for analysts, engineers, and operators.

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## Why Spinner?

* **Fragment-to-fabric reasoning.** Spinner listens to shell transcripts, notebook results, tickets, and ad-hoc text, and then spins them into a coherent task plan.
* **One runtime, many surfaces.** The CLI/TUI, HTTP+SSE server, and Next.js dashboard consume the same streaming Think→Act→Observe events.
* **Context-aware tooling.** File, shell, search, browser, and notebook helpers auto-route through the sandbox runtime when configured.
* **Observability-first.** Structured logs, OpenTelemetry traces, Prometheus metrics, and per-session cost tracking ship with the agent.
* **Evaluation harness.** SWE-Bench automation plus batch runners ensure Spinner’s weaving stays measurable.

---

## Architecture Snapshot

```
Delivery (CLI, Server, Web) → Agent Application Layer → Domain Ports → Infrastructure Adapters
```

| Layer | Highlights |
| --- | --- |
| Delivery | `cmd/alex`, `cmd/alex-server`, `web/` share the same DI container for consistent behavior. |
| Agent core | `internal/agent/app`, `internal/agent/domain`, `internal/agent/ports` implement the ReAct loop, approvals, and typed events. |
| Infrastructure | `internal/di`, `internal/tools`, `internal/toolregistry`, `internal/llm`, `internal/mcp`, `internal/session`, `internal/storage`, `internal/observability`, `internal/context` provide adapters, LLM clients, MCP runtime, persistence, rendering, telemetry, and layered context. |
| Frontend | `web/` renders real-time sessions via SSE, supports cost inspection, and lets operators feed new fragments. |

See [`docs/AGENT.md`](docs/AGENT.md) for a deep dive into the orchestration flow.

---

## Context Design

Spinner treats context like a woven fabric:

1. **Strands** (raw snippets) flow into `internal/context` where they are tagged with provenance (file path, tool, timestamp).
2. **Braids** (layered prompts) merge system instructions, conversation history, retrieved memory, and active tool constraints.
3. **Loom policies** in `internal/agent/domain` decide when to expand, compress, or fork memory using `internal/session`, `internal/context`, and `internal/rag`.
4. **Observation wefts** are streamed through `internal/output` so every surface (CLI, server, web) watches the same evolving plan.
5. **Memory compression** passes through the context builder to keep the woven net light enough for fast LLM calls while retaining anchors.

The result: fragments become a searchable lattice you can reuse later.

---

## Delivery Surfaces

* **Interactive CLI/TUI (`cmd/alex`).** Bubble Tea interface with streaming reasoning, transcript export, and preset selection.
* **HTTP + SSE server (`cmd/alex-server`).** Exposes `/tasks`, `/sessions`, `/health`, and streaming updates for automation and the dashboard.
* **Web dashboard (`web/`).** Next.js 14 app with Tailwind CSS, real-time task feed, and cost/session inspection.

All binaries wire through `internal/di`, so configuration, tool wiring, and sandbox routing stay identical.

---

## Tooling & Integrations

* **LLM providers.** Multi-model clients with retry/cost middleware in `internal/llm`.
* **Sandbox runtime.** Optional remote execution via `third_party/sandbox-sdk-go`, orchestrated through `internal/di` and tool adapters.
* **Model Context Protocol.** JSON-RPC 2.0 clients, supervisors, and server registry in `internal/mcp` so Spinner can negotiate external tools.
* **Approval workflows.** Guardrails in `internal/approval` let humans veto risky tool usage.
* **Context & parsing.** Layered prompt builders plus structured tool call parsers in `internal/context` and `internal/parser`.
* **Evaluations.** `evaluation/` hosts SWE-Bench runners, reporting helpers, and reproducible scripts.

---

## Repository Map

| Path | Purpose |
| --- | --- |
| `cmd/` | Go entrypoints (`alex`, `alex-server`). |
| `internal/` | Agent core, DI, tools, context orchestration, storage, and observability. |
| `web/` | Next.js dashboard with SSE client, session list, and controls. |
| `evaluation/` | SWE-Bench automation and reporting utilities. |
| `docs/` | Architecture notes, references, operations guides, and research. |
| `tests/` | End-to-end and integration suites executed in CI. |
| `scripts/` | Developer automation and CI helpers. |
| `deploy/docker/` | Dockerfiles, Compose stacks, and nginx config for containerized deployments. |
| `third_party/` | Vendored or customized dependencies (e.g., sandbox SDK). |

---

## Getting Started

### Prerequisites

* Go **1.24+** (pinned in `go.mod`).
* Node.js **20+** for the dashboard.
* Docker for optional sandbox services and deployments.

### CLI Quickstart

```bash
make build        # build ./alex
./alex            # launch the Spinner TUI
./alex --no-tui   # run in legacy line-mode
```

Session cleanup helpers:

```bash
./alex sessions cleanup --older-than 30d --keep-latest 25
./alex sessions cleanup --older-than 14d --dry-run
```

### Server & Dashboard

```bash
./alex-server           # start HTTP + SSE server on port 8000
(cd web && npm install)  # install frontend dependencies
(cd web && npm run dev)  # launch Next.js dashboard
```

Export `SANDBOX_BASE_URL` (or set it in `~/.alex-config.json`) to enable sandbox routing. `docker compose up` bind-mounts your host configuration into the container so Spinner reads the same credentials everywhere.

### Production Deployment

1. **Configure secrets and models:** Populate `api_key`, `base_url`, and `model` in `~/.alex-config.json`, and export env vars like `OPENAI_API_KEY` and `TAVILY_API_KEY`.
2. **One-shot startup:** Run `./deploy.sh` with no arguments to bring up nginx on port 80 with a same-origin front/back end (no need to expose 8000). On mainland China networks, `./deploy.sh cn` switches Docker/npm/Go mirrors and preloads the Sandbox image. Use `./deploy.sh pro status`/`pro logs`/`pro down` for inspections, logs, and shutdowns; use the separate `./dev.sh` for local development mode.
3. **Containerized deployment:** Prefer `deploy/docker/docker-compose.yml` for production:
   ```bash
   echo "OPENAI_API_KEY=sk-your-key" > .env
   docker compose -f deploy/docker/docker-compose.yml up -d
   docker compose -f deploy/docker/docker-compose.yml logs -f alex-server
   ```
   See the Docker Compose section of the **ALEX Operations Guide** for more commands and health-check examples.
4. **Health probes and observability:** Configure a `/health` probe (Compose or K8s livenessProbe) before go-live and wire logs/metrics into your monitoring stack; details live in the operations guide's health monitoring section.
5. **Environment-specific tuning:** Use `dev.json` / `staging.json` / `prod.json` or environment variables to enable/disable MCP, adjust logging, and tweak iteration counts; avoid debug logging in production.

### Deployment Scripts

`deploy.sh` handles the nginx-backed production stack (hydrating secrets from `~/.alex-config.json`, override with `ALEX_CONFIG_PATH`). `dev.sh` is now dedicated to local development. Key helpers:

```bash
./deploy.sh
./deploy.sh pro up
./deploy.sh pro logs web
./deploy.sh pro down
./deploy.sh cn deploy
./dev.sh start
./dev.sh test
```

Set `COMPOSE_FILE=/path/to/compose.yml` to target a different stack definition for production.

### Development Workflow

```bash
make dev     # format, vet, build, and test Go modules
make test    # execute Go unit and integration tests
make fmt     # gofmt + goimports
```

Frontend commands live in `web/README.md`; evaluation jobs use scripts in `evaluation/`.

---

## Roadmap

* **TUI polish.** Improve inline help, command palette hints, and transcript exports to reduce onboarding friction.
* **Server hardening.** Expand health and readiness probes plus structured error responses; K8s manifests already exist, observability dashboards are planned.
* **Memory accuracy.** Tighten retrieval and summarization loops in `internal/context` and `internal/rag`; compression tuning is in progress.
* **Tool safety.** Keep approvals mandatory for destructive actions; configurable policy templates are planned for broader deployment profiles.
* **Evaluation coverage.** Continue growing SWE-Bench and regression suites under `evaluation/` and `tests/` to track weaving quality.

Items labeled “planned” are in active design; other bullets align with the current codebase and deployment scripts.

---

## Documentation

* [`docs/README.md`](docs/README.md) – full documentation index.
* [`docs/AGENT.md`](docs/AGENT.md) – reasoning loop, orchestration flow, and event model.
* [`docs/architecture/`](docs/architecture/) – design deep dives and diagrams.
* [`docs/reference/`](docs/reference/) – API references, presets, formatting, observability, and MCP guides.
* [`docs/guides/`](docs/guides/) – task-focused walkthroughs.
* [`docs/operations/`](docs/operations/) – deployment, release, and monitoring guides.

---

## Contributing

1. Fork and clone the repository.
2. Run `make dev` to ensure everything builds and tests pass.
3. Follow the formatting standards in `docs/reference/FORMATTING_GUIDE.md`.
4. Open a pull request with clear commits and relevant docs or tests.

---

## License

Spinner (ALEX) is released under the [MIT License](LICENSE).
