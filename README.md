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

ALEX keeps its core reasoning loop isolated from infrastructure via clear interfaces. Delivery layers (CLI, HTTP/SSE, web) call
into application services that coordinate the agent, tools, and state management.

### High-level components

| Component | Description |
|-----------|-------------|
| `cmd/alex` | Entry point for the interactive CLI and terminal UI. Wires configuration, builds the DI container, and renders agent output. |
| `cmd/alex-server` | Minimal binary exposing the HTTP + SSE API used by the web frontend and automations. |
| `internal/agent` | Domain logic for the ReAct agent, presets, iteration orchestration, and typed events streamed to renderers. |
| `internal/tools` | Built-in tool implementations and registry (`builtin/`), including safe wrappers for shell, file, search, and reasoning actions. |
| `internal/llm` | Multi-provider clients with pricing metadata, retry logic, and cost decorators. |
| `internal/mcp` | Model Context Protocol client, JSON-RPC transport, and tool adapters for external capability providers. |
| `internal/session` | Persistent transcript storage (file-backed) with compression and indexing helpers. |
| `internal/rag` | Chunking, embedding, retrieval, and store abstractions for retrieval-augmented workflows. |
| `internal/output` | Renderers for CLI, SSE, and LLM-friendly output streams. |
| `internal/observability` | Logging, metrics, tracing, and instrumentation configuration. |
| `internal/config` | Loading, validation, and persistence for `~/.alex-config.json`, environment overrides, and feature flags. |
| `internal/di` | Dependency injection container that builds and starts subsystems lazily based on configuration. |
| `internal/server` | Application coordinators, HTTP handlers, and ports used by `alex-server`. |
| `evaluation/` | SWE-Bench and agent benchmarking harnesses. |
| `web/` | Next.js application providing the real-time dashboard. |

```
internal/
├── agent/           # domain entities, application coordinators, and ports
├── config/          # runtime configuration loaders and savers
├── context/         # session-scoped context manager
├── di/              # container wiring and feature flag toggles
├── llm/             # provider clients, pricing, retry/cost middleware
├── mcp/             # MCP client, registry, and JSON-RPC infrastructure
├── observability/   # logging, metrics, tracing setup
├── output/          # CLI + SSE renderers and formatting utilities
├── rag/             # chunking, embedding, retrieval interfaces
├── server/          # HTTP/SSE coordinators, handlers, and ports
├── session/         # file-backed session persistence
├── storage/         # cost tracking persistence adapters
├── tools/           # builtin tool implementations & registry
└── utils/           # shared helpers (paths, errors, validation)
```

The dependency direction flows inward: delivery layers depend on `internal/agent` ports and coordinators, which depend on
provider interfaces defined in `internal/agent/ports`. Infrastructure packages implement those interfaces and are registered
via the DI container.

Additional design documentation lives under `docs/architecture/` and `docs/reference/` for deeper context (architecture review,
foundational components, MCP guide, etc.).

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
```

The server exposes:

- `POST /api/tasks` – submit work to the agent (supports presets and MCP flags)
- `POST /api/tasks/{id}/cancel` – cancel a running task
- `GET /api/sessions/{id}/cost` – retrieve per-session spend
- `GET /api/sse?session_id=<id>` – stream events for a session
- `GET /health` – readiness and liveness checks

### Docker Compose

```bash
docker-compose up -d            # start Go backend + Next.js frontend
```

Set credentials in `.env` (e.g. `OPENAI_API_KEY`, provider endpoints) before starting containers.

### Testing

```bash
make test                       # run Go unit tests
npm --prefix web test           # run Vitest suite for the web app
npm --prefix web run test:e2e   # execute Playwright end-to-end tests
```

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
