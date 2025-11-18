# ALEX

ALEX (Agile Light Easy Xpert) is a terminal-native AI programming agent that runs a layered Go backend with a modern web dashboard. The project keeps the core reasoning loop independent from delivery concerns so you can drive the agent from the CLI, HTTP/SSE APIs, or the Next.js UI without duplicating logic.

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## ✨ Highlights

- **Unified ReAct runtime** – a streaming Think→Act→Observe loop that orchestrates tools, approvals, and memory.
- **Multi-surface delivery** – interactive TUI/CLI (`cmd/alex`), HTTP + SSE server (`cmd/alex-server`), and a real-time web dashboard (`web/`).
- **Sandbox-aware tools** – file, shell, search, browser, and notebook helpers that automatically route through the managed sandbox runtime when configured.
- **MCP ecosystem** – JSON-RPC 2.0 client, supervisor, and registry for external Model Context Protocol servers.
- **Observability-first** – structured logging, OpenTelemetry tracing, Prometheus metrics, and per-session cost tracking.
- **Evaluation harness** – SWE-Bench runner, batch execution helpers, and reproducible reporting utilities (`evaluation/`).

---

## 🧱 Architecture Overview

ALEX is organised as a layered system. Delivery binaries depend on application services exposed by the agent core, while infrastructure packages implement the ports required by that core. Frontend clients consume the same streaming events exposed over SSE.

```
Delivery (CLI, Server, Web) → Agent Application Layer → Domain Ports → Infrastructure Adapters
```

| Layer | Key Packages | Responsibilities |
|-------|--------------|------------------|
| Delivery | `cmd/alex`, `cmd/alex-server`, `web/` | CLI + TUI, HTTP & SSE APIs, and the Next.js dashboard.
| Agent core | `internal/agent/app`, `internal/agent/domain`, `internal/agent/ports` | ReAct coordinators, domain entities, iteration policies, typed event stream.
| Infrastructure | `internal/di`, `internal/tools`, `internal/toolregistry`, `internal/llm`, `internal/mcp`, `internal/session`, `internal/storage`, `internal/output`, `internal/observability`, `internal/prompts`, `internal/environment`, `internal/diagnostics` | Dependency wiring, tool implementations, LLM clients, MCP runtime, persistence, rendering, telemetry, and environment capture.
| Frontend | `web/` | Next.js app that subscribes to SSE, renders sessions, and submits work.

See [`docs/AGENT.md`](docs/AGENT.md) for a deeper walkthrough of the reasoning loop and orchestration flow.

---

## 🚦 Delivery Surfaces

- **Interactive CLI/TUI** – Bubble Tea interface with streaming reasoning, transcript export, and preset selection.
- **HTTP + SSE server** – exposes `/tasks`, `/sessions`, `/health`, and streaming updates for automation and the dashboard.
- **Web dashboard** – Next.js 14 app with Tailwind CSS, real-time task feed, and cost/session inspection.

All surfaces share the same dependency injection container (`internal/di`) so configuration, tool wiring, and sandbox routing behave consistently.

---

## 🤖 Agent Runtime

1. Configuration is loaded (`internal/config`) and the DI container wires LLM providers, tool registries, MCP servers, storage, and observability.
2. The agent application layer (`internal/agent/app`) coordinates task execution using domain policies (`internal/agent/domain`).
3. Each iteration streams observations through typed events consumed by renderers (`internal/output`) or SSE encoders (`internal/server`).
4. Tool calls resolve through the registry (`internal/toolregistry`) and respect sandbox mode abstractions in `internal/tools`.
5. Sessions, memory compression, and retrieval live in `internal/session`, `internal/context`, and `internal/rag` for long-running tasks.
6. Costs, metrics, traces, and structured logs are emitted through `internal/observability` and `internal/storage`.

---

## 🔌 Integrations & Tooling

- **LLM providers** – multi-model support with retry/cost middleware in `internal/llm`.
- **Sandbox runtime** – optional remote execution provided by `third_party/sandbox-sdk-go`, orchestrated via `internal/di` and tool adapters.
- **MCP** – JSON-RPC 2.0 clients, process supervisor, and server registry in `internal/mcp`.
- **Approval workflows** – user confirmation prompts and guardrails in `internal/approval`.
- **Prompts & parsing** – prompt builders and structured tool call parsing in `internal/prompts` and `internal/parser`.
- **Evaluations** – SWE-Bench runner, reporting, and helper scripts in `evaluation/`.

---

## 📁 Repository Layout

| Path | Purpose |
|------|---------|
| `cmd/` | Go entrypoints (`alex`, `alex-server`). |
| `internal/` | Backend implementation (agent core, DI, tools, LLM, storage, observability, prompts, etc.). |
| `web/` | Next.js dashboard with SSE client, task submission UI, and Playwright/Vitest tests. |
| `evaluation/` | SWE-Bench automation, batch execution helpers, and reporting utilities. |
| `docs/` | Architecture notes, reference material, operational guides, and research. |
| `tests/` | End-to-end and integration suites executed in CI. |
| `scripts/` | Developer automation and CI helpers. |
| `third_party/` | Vendored or customised dependencies (e.g. sandbox SDK). |

A curated documentation map lives in [`docs/README.md`](docs/README.md).

---

## 🚀 Getting Started

### Prerequisites

- Go **1.24+** (toolchain pinned in `go.mod`).
- Node.js **20+** for the web dashboard.
- Docker (optional) for sandbox services and containerised deployment.

### CLI Quickstart

```bash
make build        # build ./alex
./alex            # launch the interactive TUI
./alex --no-tui   # run in legacy line-mode
```

#### Session cleanup

Use the CLI to prune historical session files and free disk space:

```bash
./alex sessions cleanup --older-than 30d --keep-latest 25   # delete everything older than 30 days, keep 25 newest
./alex sessions cleanup --older-than 14d --dry-run          # preview the impact without deleting
```

### Server & Dashboard

```bash
./alex-server           # start HTTP + SSE server on default port 8000
(cd web && npm install)  # install frontend dependencies
(cd web && npm run dev)  # launch Next.js dashboard with live SSE
```

To enable the sandbox runtime, export `SANDBOX_BASE_URL` or set it in `~/.alex-config.json`. The DI container will wire the shared `SandboxManager` for every surface.

When running `docker compose up`, the `alex-server` service bind-mounts your host `~/.alex-config.json` into `/root/.alex-config.json` inside the container so the server automatically picks up the same credentials and model configuration as your local CLI.

### Development Workflow

```bash
make dev     # format, vet, build, and run tests
make test    # execute Go unit and integration tests
make fmt     # gofmt + goimports across the repo
```

Frontend commands live under `web/` (see `web/README.md` for details). Evaluation jobs are orchestrated with scripts under `evaluation/`.

---

## 📚 Documentation

- [`docs/README.md`](docs/README.md) – documentation index and navigation.
- [`docs/AGENT.md`](docs/AGENT.md) – agent runtime, reasoning loop, and event flow.
- [`docs/architecture/`](docs/architecture/) – design deep dives and diagrams.
- [`docs/reference/`](docs/reference/) – API references, presets, formatting, observability, and MCP guides.
- [`docs/guides/`](docs/guides/) – task-focused walkthroughs (SSE, acceptance tests, etc.).
- [`docs/operations/`](docs/operations/) – deployment, release, and monitoring guides.

---

## 🤝 Contributing

1. Fork and clone the repository.
2. Run `make dev` to ensure everything builds and tests pass.
3. Follow the coding standards in `docs/reference/FORMATTING_GUIDE.md`.
4. Open a pull request with clear commits and include relevant docs or tests.

---

## 📄 License

ALEX is released under the [MIT License](LICENSE).
