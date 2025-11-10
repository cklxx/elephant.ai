# ALEX

Terminal-native AI programming agent built in Go with a modern web companion UI.

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Overview

ALEX (Agile Light Easy Xpert) is a privacy-first coding copilot focused on terminal workflows. The Go backend drives a ReAct
agent that can stream its reasoning, invoke a rich tool suite, persist context, and expose capabilities through a CLI, HTTP API,
and a real-time web dashboard. The frontend (Next.js + Tailwind CSS) mirrors task execution and session history while staying
synchronized via Server-Sent Events (SSE).

Key goals:

- Keep domain logic independent from delivery concerns with a layered, hexagonal architecture.
- Provide first-class ergonomics for terminal users through an interactive TUI and CLI commands.
- Support multiple LLM providers, cost tracking, Model Context Protocol (MCP) integrations, and reproducible evaluations.

## Capabilities

- **Terminal experience** – bubbletea-based TUI, session management, transcript export, and legacy non-TUI CLI support
  (`cmd/alex`).
- **HTTP & SSE API** – streaming task execution, cancellation, health checks, and cost inspection (`cmd/alex-server`,
  `internal/server`).
- **Modern web dashboard** – Next.js 14 application with live SSE updates, task submission, and session history (`web/`).
- **Secure sandbox execution** – remote filesystem, shell, browser, and notebook operations routed through a managed sandbox
  runtime (`internal/tools`, `internal/di`, `internal/server/app`).
- **Tooling ecosystem** – 15+ built-in tools for file operations, shell execution, search, TODO management, reasoning helpers,
  and web retrieval (`internal/tools/builtin`).
- **MCP integration** – JSON-RPC 2.0 client, process supervisor, and registry for external MCP servers (`internal/mcp`).
- **Session & memory** – compressed transcript storage, resumable sessions, and RAG pipelines for long-term recall
  (`internal/session`, `internal/context`, `internal/rag`).
- **Observability & cost controls** – Prometheus metrics, OpenTelemetry tracing, structured logging, per-session spend tracking,
  and exportable cost reports (`internal/observability`, `internal/output`, `internal/storage`).
- **Evaluation harness** – SWE-Bench runner, batch execution helpers, and reporting utilities (`evaluation/`).

## Sandbox Execution Environment

ALEX now ships with a fully managed sandbox runtime that isolates agent actions from the host machine while preserving the
rich toolset developers expect. When a sandbox endpoint is configured, the backend negotiates a shared `SandboxManager`
and transparently routes file, shell, browser, and Jupyter traffic through the remote execution service.

### Highlights

- **Isolated compute** – Commands are executed remotely with per-session namespaces and guardrails enforced by the sandbox
  SDK (`internal/tools/builtin/bash.go`, `internal/tools/builtin/file_edit.go`).
- **Full tool coverage** – All filesystem and search helpers understand sandbox mode, including `find`, `ripgrep`,
  multi-file edits, and binary uploads (`internal/tools/builtin`, `third_party/sandbox-sdk-go`).
- **Environment snapshots** – Host and sandbox environment variables are captured and streamed to clients to aid debugging
  (`internal/prompts/environment_summary.go`, `internal/agent/domain/events.go`).
- **Health & telemetry** – A dedicated probe verifies connectivity and surfaces availability alongside LLM and MCP status
  in health checks and metrics (`internal/server/app/health.go`).
- **Graceful fallback** – If initialization fails the runtime automatically reverts to local execution while warning the user
  (`internal/di/container.go`).

### Enable the sandbox

Set the sandbox endpoint once and every delivery surface (CLI, HTTP API, and web dashboard) will use remote execution:

```bash
export SANDBOX_BASE_URL="https://sandbox.example.com"
./alex-server            # automatically boots in sandbox mode
./alex --config show     # confirms execution_mode: sandbox
```

The same value can be persisted in `~/.alex-config.json` under `sandbox_base_url`. The dependency injection container wires
the shared manager during startup, performs a connectivity check, and exposes the configured URL to downstream packages for
tool registry, observability, and diagnostics (`internal/di/container.go`, `internal/toolregistry/registry.go`).

### Operating the sandbox

- Use `GET /health` (or `make server-health`) to verify the sandbox component status advertised by the probe.
- Monitor latency and failure counters exported via the sandbox Prometheus metrics (`docs/operations/monitoring_and_metrics.md`).
- Capture environment differentials inside transcripts to reproduce sandbox-only issues from the CLI or dashboard.

## Architecture

ALEX is built as a layered system so the core reasoning loop can evolve independently from delivery surfaces or infrastructure. Command binaries and the web dashboard depend on the agent application services, which in turn orchestrate infrastructure packages through clearly defined ports.

### High-level components

| Area | Component | Description |
|------|-----------|-------------|
| Delivery | `cmd/alex` | Interactive CLI + terminal UI entrypoint. Loads configuration, builds the DI container, and streams agent output. |
| Delivery | `cmd/alex-server` | HTTP + SSE server used by the dashboard and automations. Shares the same container wiring as the CLI. |
| Agent core | `internal/agent/app` | Coordinators that drive task execution, schedule tool calls, and surface typed events for renderers. |
| Agent core | `internal/agent/domain` | Pure ReAct loop, iteration policies, and domain entities used across delivery layers. |
| Agent core | `internal/agent/ports` | Interfaces consumed by the domain layer for LLMs, tools, storage, approvals, and streaming output. |
| Infrastructure | `internal/di` | Dependency injection container that lazily wires infrastructure (LLMs, tool registry, sandbox manager, storage). |
| Infrastructure | `internal/tools` + `internal/toolregistry` | Built-in tool implementations, sandbox-aware execution mode abstractions, and metadata registry. |
| Infrastructure | `internal/llm` | Provider clients with retry logic, streaming support, pricing metadata, and cost decorators. |
| Infrastructure | `internal/mcp` | Model Context Protocol process supervisor, registry, and JSON-RPC transport. |
| Infrastructure | `internal/session` + `internal/storage` | Transcript persistence, compression, and cost tracking stores. |
| Infrastructure | `internal/output` + `internal/observability` | Renderers, logging, metrics, tracing, and telemetry helpers. |
| Infrastructure | `internal/prompts`, `internal/parser`, `internal/environment`, `internal/diagnostics` | Prompt assembly, structured tool-call parsing, environment summaries, and health probes shared by the agent surfaces. |
| Frontend | `web/` | Next.js dashboard subscribing to SSE streams with task submission UI and history views. |
| Tooling | `evaluation/` | SWE-Bench runner and automated evaluation harnesses. |

```
internal/
├── agent/           # domain entities, presets, application coordinators, and ports
├── approval/        # user confirmation + guardrail prompts
├── config/          # runtime configuration loaders and savers
├── context/         # session-scoped context manager
├── diagnostics/     # health probes and sandbox progress reporting
├── diff/            # diff rendering utilities for file patches
├── di/              # container wiring and lifecycle management
├── environment/     # environment capture + summarization helpers
├── errors/          # shared error helpers and sentinels
├── integration/     # cross-cutting integration tests and adapters
├── llm/             # provider clients, retry/cost middleware, factories
├── mcp/             # MCP client, registry, and JSON-RPC infrastructure
├── observability/   # logging, metrics, tracing setup
├── output/          # CLI + SSE renderers and formatting utilities
├── parser/          # structured tool/function call parsing helpers
├── prompts/         # system prompt construction and templates
├── rag/             # retrieval augmented generation components
├── security/        # redaction and safety policies
├── server/          # HTTP/SSE coordinators, handlers, and ports
├── session/         # file-backed session persistence
├── storage/         # cost tracking persistence adapters
├── toolregistry/    # tool registry metadata and filtering
├── tools/           # builtin tool implementations & sandbox clients
└── utils/           # shared helpers (paths, formatting, validation)
```

The dependency direction flows inward: delivery layers depend on `internal/agent` ports and coordinators, which depend on provider interfaces defined in `internal/agent/ports`. Infrastructure packages implement those interfaces and are registered via the DI container when the binaries start up.

For additional diagrams and deep dives see the curated docs in `docs/architecture/` (sandbox migration, web UI service design) and reference material in `docs/reference/`.

## Repository Layout

| Path | Purpose |
|------|---------|
| `cmd/` | Go entrypoints (`alex`, `alex-server`). |
| `internal/` | Core backend implementation (agent, tools, llm, session, observability, etc.). |
| `evaluation/` | Benchmarks, SWE-Bench runner, and experiment tooling. |
| `web/` | Next.js frontend with SSE dashboard and Playwright/Vitest tests. |
| `docs/` | Architecture notes, ADRs, design specs, and operational guides. |
| `tests/` | End-to-end and integration test suites (Go). |
| `scripts/` | Developer and CI automation helpers. |
| `npm/` | Pre-built binaries packaged for npm distribution. |

## Getting Started

### Prerequisites

- Go **1.24+** (toolchain pinned in `go.mod`)
- Node.js **20+** for the web UI (optional if using CLI only)
- Docker (optional) for containerized deployment

### Build and run the CLI

```bash
make build           # build ./alex
./alex               # launch interactive TUI
./alex --no-tui      # legacy line-mode CLI
./alex session list  # inspect stored sessions
```

Configuration is stored in `~/.alex-config.json`. Use `./alex config show` to inspect, or export overrides via environment
variables (e.g. `OPENAI_API_KEY`, `ALEX_ENABLE_MCP`).

### Run the server + web dashboard

```bash
make server-run                 # start alex-server on :8080
(cd web && npm install)
(cd web && npm run dev)         # start Next.js dev server on :3000
# or run ./deploy.sh to build the backend, launch the web dev server locally,
# and automatically start the sandbox container in Docker
./deploy.sh
```

The server exposes:

- `POST /api/tasks` – submit work to the agent (supports presets and MCP flags)
- `POST /api/tasks/{id}/cancel` – cancel a running task
- `GET /api/sessions/{id}/cost` – retrieve per-session spend
- `GET /api/sse?session_id=<id>` – stream events for a session
- `GET /health` – readiness and liveness checks

### Docker Compose

```bash
docker-compose up -d            # start Go backend, sandbox runtime, and Next.js frontend
./deploy.sh docker up           # auto-detect docker compose and start the stack
```

Set credentials in `.env` (e.g. `OPENAI_API_KEY`, provider endpoints) before starting containers.
The compose stack now launches a dedicated `alex-sandbox` container and configures the
server with `ALEX_SANDBOX_BASE_URL=http://alex-sandbox:8080` so file/shell/code tools use the
isolated sandbox runtime while retaining the shared `skills/` guides inside that container.

> **China Mirror Acceleration:** For 🚀 **instant startup** in mainland China (30s vs 15-25min):
> ```bash
> ./scripts/setup-china-mirrors-all.sh  # One-click setup (uses Volcengine pre-built image)
> # Or manually add to .env: SANDBOX_IMAGE=enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest
> ```
> See [`docs/deployment/CHINA_MIRRORS.md`](docs/deployment/CHINA_MIRRORS.md) for details.

### Testing

```bash
make test                       # run Go unit tests
npm --prefix web test -- --run  # run Vitest suite for the web app (non-watch mode)
npm --prefix web run e2e        # execute Playwright end-to-end tests
./deploy.sh test                # run all of the above sequentially
```

> **Note:** On a fresh machine, install Playwright browsers once via
> `npx --prefix web playwright install` (add `--with-deps` on Linux if system
> libraries are missing).

Evaluation jobs can be executed via:

```bash
./alex run-batch --dataset.subset lite --workers 4 --output ./results
```

## Further Reading

- [`docs/reference/ALEX.md`](docs/reference/ALEX.md) – comprehensive project reference (architecture, development, operations).
- [`docs/architecture/SPRINT_1-4_ARCHITECTURE.md`](docs/architecture/SPRINT_1-4_ARCHITECTURE.md) – architecture review and sprint outcomes.
- [`web/README.md`](web/README.md) – details on the Next.js dashboard.
- [`docs/reference/MCP_GUIDE.md`](docs/reference/MCP_GUIDE.md) – integrating external MCP tools.

Contributions are welcome! Please check the docs and tests for guidance on extending tools, adding providers, or improving the
TUI/web experiences.
