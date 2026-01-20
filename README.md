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
# Configure your LLM provider (example: OpenAI)
export OPENAI_API_KEY="sk-..."
cp examples/config/runtime-config.yaml ~/.alex/config.yaml

# Run backend + web together
./dev.sh

# Check status/logs
./dev.sh status
./dev.sh logs server
./dev.sh logs web

# Stop services
./dev.sh down

# Optional: build and run the CLI/TUI
make build
./alex
./alex "print the repo layout"
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

## Context-first multi-agent system

elephant.ai ships a **context-first** multi-agent system. The main service plays the role of the **architect** that manages shared context, priorities, and compression. Each executor runs inside its own VM (or container sandbox), acting as the **doer** with deterministic tool access. The system is designed to keep shared context consistent across all CLIs, including Codex, Claude Code, Gemini CLI, and other compatible clients.

Key ideas:

* **Context sharing with explicit priority:** system > task > session > tool > scratch. Higher-priority context is preserved across agents and delivery surfaces.
* **Automatic compression:** low-priority context is summarized and compacted before overflow, so the architect can always route the right context to executors.
* **Architect ↔ executor separation:** the architect focuses on planning, delegation, and consolidation; executors focus on deterministic tool runs inside VMs.
* **Client-neutral transport:** any CLI that speaks the runtime protocol can connect without losing context fidelity.

```mermaid
flowchart LR
  subgraph Delivery
    CLI1[Codex CLI] --> Gateway
    CLI2[Claude Code] --> Gateway
    CLI3[Gemini CLI] --> Gateway
    CLI4[Other CLIs] --> Gateway
  end

  Gateway[Shared Runtime Gateway] --> Architect[Main Service Architect]

  Architect -->|prioritized context| ContextStore[(Shared Context Store)]
  ContextStore -->|context slices| Architect
  Architect -->|task plan| Executor1[VM Executor A]
  Architect -->|task plan| Executor2[VM Executor B]

  subgraph Executors
    Executor1 --> Tools1[Deterministic Tools]
    Executor2 --> Tools2[Deterministic Tools]
  end

  ContextStore -->|auto compression| Compressor[Auto Compressor]
  Compressor -->|summaries + pruning| ContextStore
```

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
