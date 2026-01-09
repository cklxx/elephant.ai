<p align="center">
  <span style="display:inline-flex; align-items:center; justify-content:center; padding:14px; border-radius:28px; background:radial-gradient(circle at 30% 30%, #34d39933, #3b82f633 55%, transparent 70%), linear-gradient(135deg, #0f172a, #111827); box-shadow:0 20px 60px -32px rgba(15, 23, 42, 0.6), 0 10px 30px -24px rgba(52, 211, 153, 0.45);">
    <span style="display:inline-flex; align-items:center; justify-content:center; width:92px; height:92px; border-radius:24px; background:linear-gradient(135deg, rgba(255,255,255,0.04), rgba(255,255,255,0)); border:1px solid rgba(255,255,255,0.08); box-shadow:inset 0 1px 0 rgba(255,255,255,0.06);">
      <img src="web/public/elephant.jpeg" alt="elephant.ai mascot" width="76" height="76" style="border-radius:20px; object-fit:cover; box-shadow:0 10px 32px -24px rgba(0,0,0,0.45);" />
    </span>
  </span>
</p>

# elephant.ai

elephant.ai is a shared Go runtime (with a Next.js dashboard) that powers the `alex` CLI/TUI, `alex-server`, and the web UI. All entrypoints run the same Think → Act → Observe loop, share a single dependency injection container, and emit typed events for consistent rendering across terminals and the browser.

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## Highlights

* One runtime, three entrypoints (CLI/TUI, server, dashboard) backed by the same DI container.
* Configurable OpenAI-compatible LLM providers with shared runtime config across CLI/server/web.
* Typed, artifact-aware events that render identically in terminals and the web UI.
* Built-in observability: structured logs, OpenTelemetry traces, Prometheus metrics, and per-session cost accounting.
* Retrieval layers for memory, skills, docs, and external context plus approvals for risky actions.
* Evaluation harnesses (including SWE-Bench) live in-repo for parity between manual and automated runs.

---

## Quickstart

Prerequisites: Go 1.24+, Node.js 20+ (web UI), Docker (optional).

```bash
# Build CLI and server
make build            # alex
make server-build     # alex-server

# Configure your LLM provider (example: OpenAI)
export OPENAI_API_KEY="sk-..."
cp examples/config/runtime-config.yaml ~/.alex/config.yaml

# Run the CLI/TUI
./alex
./alex "print the repo layout"

# Run the server (SSE + API)
make server-run

# Run the dashboard (development)
(cd web && npm install)
(cd web && npm run dev)
```

Configuration is shared across every surface. Use `examples/config/runtime-config.yaml` and `docs/reference/CONFIG.md` for the canonical schema.

---

## Architecture (short form)

```
Delivery (CLI, Server, Web) → Agent Application Layer → Domain Ports → Infrastructure Adapters
```

* Delivery: shared DI wiring for `cmd/alex`, `cmd/alex-server`, and `web/` keeps sessions consistent.
* Agent core: `internal/agent/{app,domain,ports}` implements the Think → Act → Observe loop, approvals, and typed events.
* Infrastructure: `internal/di`, `internal/tools`, `internal/toolregistry`, `internal/llm`, `internal/mcp`, `internal/session`, `internal/storage`, `internal/observability`, and `internal/context` provide adapters, context, telemetry, and persistence.
* Frontend: `web/` streams SSE events, artifacts, approvals, and cost details in real time.

---

## Key paths

* `internal/agent/`: ReAct loop, approvals, and event model.
* `internal/context/` + `internal/rag/`: layered retrieval and summarization.
* `internal/observability/`: logs, traces, metrics, and cost accounting.
* `internal/tools/` + `internal/toolregistry/`: typed tools and safety policies.
* `evaluation/`: SWE-Bench and regression harnesses.
* `deploy/`: Docker Compose entrypoints for local and production stacks.
* `web/`: Next.js dashboard that consumes the same event stream.

---

## Project governance

* [`LICENSE`](LICENSE): MIT license.
* [`CONTRIBUTING.md`](CONTRIBUTING.md): contribution workflow and code standards.
* [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md): expected community behavior.
* [`SECURITY.md`](SECURITY.md): vulnerability reporting process.

---

## Docs

* [`docs/README.md`](docs/README.md): documentation landing page and navigation.
* [`docs/AGENT.md`](docs/AGENT.md): runtime overview covering the Think → Act → Observe loop and delivery surfaces.
* [`docs/reference/ALEX.md`](docs/reference/ALEX.md): architecture and development reference, including module boundaries and common commands.
* [`docs/reference/CONFIG.md`](docs/reference/CONFIG.md): canonical configuration schema and precedence.
* [`docs/guides/quickstart.md`](docs/guides/quickstart.md): from clone to running CLI/server/web.
* [`docs/operations/DEPLOYMENT.md`](docs/operations/DEPLOYMENT.md): deployment guide for local, Docker Compose, and custom clusters.
