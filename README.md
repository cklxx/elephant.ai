<p align="center">
  <span style="display:inline-flex; align-items:center; justify-content:center; padding:14px; border-radius:28px; background:radial-gradient(circle at 30% 30%, #34d39933, #3b82f633 55%, transparent 70%), linear-gradient(135deg, #0f172a, #111827); box-shadow:0 20px 60px -32px rgba(15, 23, 42, 0.6), 0 10px 30px -24px rgba(52, 211, 153, 0.45);">
    <span style="display:inline-flex; align-items:center; justify-content:center; width:92px; height:92px; border-radius:24px; background:linear-gradient(135deg, rgba(255,255,255,0.04), rgba(255,255,255,0)); border:1px solid rgba(255,255,255,0.08); box-shadow:inset 0 1px 0 rgba(255,255,255,0.06);">
      <img src="web/public/elephant.jpeg" alt="elephant.ai mascot" width="76" height="76" style="border-radius:20px; object-fit:cover; box-shadow:0 10px 32px -24px rgba(0,0,0,0.45);" />
    </span>
  </span>
</p>

# elephant.ai

elephant.ai is a Go runtime with a Next.js dashboard that follows a Think → Act → Observe loop. CLI/TUI (`cmd/alex`), server (`cmd/alex-server`), and web UI share the same dependency injection container and event stream.

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## Highlights

* Shared runtime across CLI/TUI, server, and dashboard.
* Typed, artifact-aware events that the dashboard can render without log scraping.
* Built-in observability: structured logs, OpenTelemetry traces, Prometheus metrics, and per-session cost tracking.
* Grounded retrieval layers for memory, skills, docs, and external context.
* Evaluation harnesses (including SWE-Bench) live in the repo.

---

## Architecture (short form)

```
Delivery (CLI, Server, Web) → Agent Application Layer → Domain Ports → Infrastructure Adapters
```

* Delivery: shared DI wiring for `cmd/alex`, `cmd/alex-server`, and `web/` keeps sessions consistent.
* Agent core: `internal/agent/{app,domain,ports}` implement the ReAct loop, approvals, and typed events.
* Infrastructure: `internal/di`, `internal/tools`, `internal/toolregistry`, `internal/llm`, `internal/mcp`, `internal/session`, `internal/storage`, `internal/observability`, and `internal/context` provide adapters, context, telemetry, and persistence.
* Frontend: `web/` streams SSE events, artifacts, approvals, and cost details in real time.

---

## Key paths

* `internal/agent/`: ReAct loop, approvals, and event model.
* `internal/context/` + `internal/rag/`: layered retrieval and summarization.
* `internal/observability/`: logs, traces, metrics, and cost accounting.
* `internal/tools/` + `internal/toolregistry/`: typed tools and safety policies.
* `evaluation/`: SWE-Bench and regression harnesses.

---

## Docs

* [`docs/README.md`](docs/README.md)
* [`docs/AGENT.md`](docs/AGENT.md)
* [`docs/reference/ALEX.md`](docs/reference/ALEX.md)
* [`docs/operations/DEPLOYMENT.md`](docs/operations/DEPLOYMENT.md)
