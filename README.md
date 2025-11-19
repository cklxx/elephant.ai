# Spinner

Spinner is an AI agent that turns scattered facts, logs, and scratch notes into an actionable knowledge web. It runs the same layered Go backend that powers ALEX, but the framing is focused on weaving together fragmented context for analysts, engineers, and operators.

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

### Unified Deployment Script

`deploy.sh` drives local development and the nginx-backed production stack while hydrating secrets from `~/.alex-config.json` (override with `ALEX_CONFIG_PATH`). Key helpers:

```bash
./deploy.sh pro up
./deploy.sh pro logs web
./deploy.sh pro down
./deploy.sh start
./deploy.sh test
```

Set `COMPOSE_FILE=/path/to/compose.yml` to target a different stack definition.

### Development Workflow

```bash
make dev     # format, vet, build, and test Go modules
make test    # execute Go unit and integration tests
make fmt     # gofmt + goimports
```

Frontend commands live in `web/README.md`; evaluation jobs use scripts in `evaluation/`.

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
