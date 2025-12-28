# elephant.ai

elephant.ai provides a Go backend and Next.js dashboard built around a shared Think → Act → Observe loop. The `alex` CLI/TUI, HTTP + SSE server, and web UI run the same runtime so operators and automation stay in sync.

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## Why it’s different

* **One runtime, many surfaces.** CLI/TUI (`cmd/alex`), server (`cmd/alex-server`), and dashboard (`web/`) all consume the same dependency injection container and event stream, keeping operator and automation behavior identical.
* **Artifact-rich, typed events.** File, shell, search, and notebook helpers emit structured events with attachments so the dashboard can render transcripts, artifacts, and approvals without brittle parsing.
* **Observation-first instrumentation.** Structured logs, OpenTelemetry traces, Prometheus metrics, and per-session cost tracking are built into the runtime via `internal/observability`.
* **Safety with intent visibility.** Destructive tools surface explicit approvals and safety policies in the ReAct loop so operators see what will happen before it runs.
* **Grounded context builder.** Layered retrieval (`internal/context`, `internal/rag`) merges session state, skills, docs, and external context to reduce hallucinations.
* **Evaluation baked in.** SWE-Bench and regression harnesses in `evaluation/` keep changes accountable during development.

---

## Design principles

* **Parity across surfaces.** The same container wiring powers CLI, server, and web so sessions are consistent regardless of entrypoint.
* **Typed interactions over ad hoc logs.** Every tool call is an event with schema-backed payloads, artifacts, and attachments rather than plain text.
* **Operator-forward UX.** The dashboard is optimized for reading and approving steps in real time through SSE streams instead of replaying logs after the fact.
* **Observability as a first-class API.** Telemetry is emitted from the agent runtime itself—not bolted onto handlers—so traces align with ReAct steps.
* **Measurable progress.** Evaluation suites live in-repo and are expected to run in development, not only in CI.

<p align="center">
  <img src="web/public/elephant.jpeg" alt="elephant.ai mascot" width="360">
</p>

---

## Architecture sketch

```
Delivery (CLI, Server, Web) → Agent Application Layer → Domain Ports → Infrastructure Adapters
```

| Layer | Highlights |
| --- | --- |
| Delivery | Shared DI container drives `cmd/alex`, `cmd/alex-server`, and `web/` for consistent sessions. |
| Agent core | `internal/agent/app`, `internal/agent/domain`, and `internal/agent/ports` implement the ReAct loop, approvals, and typed events. |
| Infrastructure | `internal/di`, `internal/tools`, `internal/toolregistry`, `internal/llm`, `internal/mcp`, `internal/session`, `internal/storage`, `internal/observability`, and `internal/context` provide adapters, LLM clients, MCP runtime, persistence, rendering, telemetry, and layered context. |
| Frontend | `web/` renders real-time sessions via SSE, surfaces cost inspection, and lets operators feed new fragments. |

---

## Components to know

| Path | Focus |
| --- | --- |
| `internal/agent/` | ReAct loop, approvals, event model, and application services. |
| `internal/context/` + `internal/rag/` | Layered retrieval and summarization for memory accuracy. |
| `internal/observability/` | Logs, traces, metrics, and cost accounting wired into the runtime. |
| `internal/tools/` + `internal/toolregistry/` | Typed tool definitions, registration, and safety policies. |
| `web/` | Next.js dashboard that streams typed events and artifacts. |
| `evaluation/` | SWE-Bench runners and regression harnesses. |

For deeper dives, see [`docs/AGENT.md`](docs/AGENT.md) and [`docs/README.md`](docs/README.md).

## Session data lifecycle

User sessions persist across turns (Plan → ReAct loops) with distinct lifetimes for each payload type:

* **Messages and turn history.** Session messages (system, user, assistant, tool results) are saved after each turn and may be auto-compressed for token budgets; system/user prompts survive compression while non-critical content can be summarized.
* **Attachments.** Binary artifacts and placeholders travel with the session and are stored separately from messages so they can be reused by later turns or delegated subagents.
* **Important notes.** High-signal, user-personalized snippets can be pinned via the `attention` tool; they share the session lifecycle with attachments and are automatically recalled and appended when history is compressed.
* **Plans and execution state.** Plan trees, beliefs, world state snapshots, and feedback signals are captured per turn so downstream UI and APIs can replay the session timeline.
* **History snapshots.** Session history is stored in the configured session store (file/SQLite/Postgres) and fed back into context windows, with compression summaries and pinned notes re-attached when token limits are hit.
