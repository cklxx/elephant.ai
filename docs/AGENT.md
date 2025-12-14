# ALEX Agent Runtime
> Last updated: 2025-11-18


This document explains how the ALEX agent executes work across the backend. It describes the reasoning loop, delivery surfaces, dependency injection container, and how infrastructure adapters plug into the domain ports.

---

## 🌍 Delivery Context

All delivery surfaces (CLI/TUI, HTTP server, and dashboard) share the same dependency injection container defined in [`internal/di`](../internal/di). On start-up each binary performs the following steps:

1. Load configuration from environment variables, config files, and flags (`internal/config`).
2. Build the DI container which registers LLM providers, tool registries, MCP servers, persistence, and observability exporters.
3. Resolve the agent application service (`internal/agent/app`) which exposes high-level commands such as `ExecuteTask`, `ResumeSession`, and `ListSessions`.
4. Attach delivery-specific renderers (Bubble Tea UI, CLI stream, HTTP handlers, SSE encoders) from `internal/output` and `internal/server`.

The uniform container ensures that enabling MCP, swapping an LLM provider, or adjusting policy applies to every surface with zero extra wiring.

---

## 🔁 ReAct Execution Loop

The agent domain (`internal/agent/domain`) models the Think→Act→Observe loop as a series of typed events:

1. **Bootstrap** – assemble the system prompt via `internal/context`, capture the environment snapshot (`internal/environment`), and hydrate session context from `internal/session` + `internal/context`.
2. **Think** – request reasoning tokens from the configured LLM provider using the `LLMPort`. Calls route through `internal/llm` which wraps provider SDKs with retry, streaming, and cost accounting middleware.
3. **Act** – parse tool intentions from the streamed tokens using `internal/parser`. When a tool invocation is confirmed, the coordinator resolves metadata from `internal/toolregistry` and dispatches execution via adapters in `internal/tools`.
4. **Observe** – capture tool output, compress logs, and emit structured events. Observations may be fed back into the short-term context or stored as long-term memory in `internal/rag` when required.
5. **Control** – iteration policies (max steps, approval gates, error handling) are defined in the domain layer. Approvals in `internal/approval` can block execution for user confirmation.
6. **Complete** – final responses, diffs, cost summaries, and metrics are streamed to renderers. Sessions are persisted and compressed for later resumption.

Every state transition produces a typed event (`internal/agent/domain/events.go`). Delivery surfaces subscribe to these events to present realtime updates without leaking domain details.

---

## 🧰 Tooling Architecture

Tools are registered through the dependency injection container:

- **Builtin tools** live in `internal/tools/builtin`. Each tool implements the `Tool` interface.
- **MCP tools** are handled by the MCP supervisor in `internal/mcp`. It manages process lifecycles, JSON-RPC connections, and exposes the tools via the same registry interface.
- **Registry filtering** (`internal/toolregistry`) applies policy decisions (allow lists, presets, read-only mode) before tools reach the domain layer.

Because tools conform to a shared interface the ReAct loop does not need to know whether a capability is local, remote, or provided via MCP.

---

## 🧠 Memory & Sessions

- **Session state** – `internal/session` persists transcripts and compressed actions. `internal/context` layers additional per-task metadata.
- **Retrieval augmented generation** – `internal/rag` manages embeddings, chunking, and retrieval for long-running projects.
- **Cost tracking** – `internal/storage` records token usage per session and exposes roll-up queries.
- **Exporters** – transcripts and diffs can be exported through tools and renderers in `internal/output`.

This separation keeps long-term memory optional while making it easy to resume or audit previous runs.

---

## 📡 Streaming & Observability

- **Event stream** – events are serialized by `internal/output` for the CLI/TUI and by `internal/server` for SSE clients.
- **Telemetry** – `internal/observability` wires structured logging, Prometheus metrics, and OpenTelemetry tracing. Tool invocations and LLM calls are instrumented.
- **Health checks** – `internal/diagnostics` exposes readiness checks and MCP status that surface via `/health`.
- **Cost & audit** – `internal/storage` and `internal/output/costs` provide per-session summaries that feed both CLI reports and the dashboard.

---

## 🔄 Evaluations

The evaluation harness in [`evaluation/`](../evaluation) reuses the same agent application service. Batch jobs spin up the container, execute tasks against SWE-Bench datasets, and emit structured reports. This guarantees parity between manual runs and automated benchmarking.

---

## 🧭 Key Flows at a Glance

```
CLI / Server / Web
        │
        ▼
 internal/di.Container
        │ resolves
        ▼
 internal/agent/app.Executor
        │ emits events
        ▼
 internal/output renderers ───► terminal / HTTP / SSE
        │
        ├─► internal/llm (LLMPort)
        ├─► internal/toolregistry → internal/tools + internal/mcp
        ├─► internal/session + internal/context
        └─► internal/observability + internal/storage
```

Use this diagram alongside the architecture docs in `docs/architecture/` for deeper component-level details.
