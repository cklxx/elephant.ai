# Architecture and Agent Execution Flow
> Last updated: 2026-01-23

This document consolidates the runtime architecture and the agent execution flow for elephant.ai.
It complements the deeper dives in `AGENTS.md`, `docs/reference/CONFIG.md`, and `docs/reference/ACP.md`.

---

## 1) System Context (Delivery Surfaces + External Dependencies)

**Delivery surfaces (shared runtime):**
- CLI/TUI: `cmd/alex`
- Server (SSE/HTTP): `cmd/alex-server`
- Web UI: `web/` (consumes the server event stream)
- ACP (Agent Client Protocol): `internal/infra/acp` (stdio + HTTP/SSE for external clients)

**External dependencies (pluggable):**
- LLM providers (OpenAI-compatible, Claude, etc.) via `internal/infra/llm`
- MCP servers (tooling over JSON-RPC) via `internal/infra/mcp`
- Storage backends (sessions, cost, telemetry) via `internal/infra/storage`
- Observability sinks (logs, metrics, traces) via `internal/infra/observability`

---

## 2) Architecture Layers (Clear Separation of Concerns)

**Delivery Layer**
- Adapters that render events to CLI/TUI, HTTP/SSE, and web.
- Key packages: `internal/delivery/output`, `internal/delivery/server`, `internal/delivery/channels`, `web/`

**Application Layer**
- Coordinates use cases and orchestrates the agent loop.
- Key packages: `internal/app/agent/coordinator`, `internal/app/context`, `internal/app/toolregistry`

**Domain Layer**
- Think -> Act -> Observe loop, events, and policies.
- Key packages: `internal/domain/agent`, `internal/domain/agent/ports`, `internal/domain/agent/types`, `internal/domain/workflow`

**Infrastructure Layer**
- Concrete adapters for LLMs, tools, MCP, sessions, Markdown memory, storage, and telemetry.
- Key packages: `internal/infra/llm`, `internal/infra/tools`, `internal/infra/mcp`,
  `internal/infra/session`, `internal/infra/memory`,
  `internal/infra/storage`, `internal/infra/observability`, `internal/shared/logging`

**Wiring and bootstrap**
- Dependency injection container: `internal/app/di`
- Configuration and environment: `internal/shared/config`, `internal/infra/environment`

![Architecture Layers](images/architecture_layers_gen.png)

---

## 3) Module Map (What Lives Where)

| Area | Responsibility | Primary Packages |
| --- | --- | --- |
| Delivery surfaces | CLI/TUI, server handlers, SSE streaming, web UI | `cmd/`, `internal/delivery/output`, `internal/delivery/server`, `web/` |
| Agent application | Use case orchestration, session commands, streaming results | `internal/app/agent/coordinator` |
| Agent domain | ReAct loop, events, policies, approvals | `internal/domain/agent`, `internal/domain/agent/ports`, `internal/shared/agent/presets` |
| LLM integration | Provider SDKs, retries, streaming, cost tracking | `internal/infra/llm`, `internal/app/subscription` |
| Tools + MCP | Built-in tools, MCP tools, registry | `internal/infra/tools`, `internal/app/toolregistry`, `internal/infra/mcp` |
| Context + memory | Short-term context, Markdown memory, session state | `internal/app/context`, `internal/infra/memory`, `internal/infra/session` |
| Observability + storage | Logs, metrics, tracing, cost storage | `internal/shared/logging`, `internal/infra/observability`, `internal/infra/storage` |
| Config + env | Config merge, env snapshot | `internal/shared/config`, `internal/infra/environment` |

---

## 4) Startup Flow (Runtime Boot Sequence)

At startup each delivery surface follows the same skeleton:
1) Load configuration from YAML + env + flags (`internal/shared/config`).
2) Snapshot environment and runtime metadata (`internal/infra/environment`).
3) Build the DI container (`internal/app/di`) and register core adapters.
4) Wire LLM providers, tool registry, MCP supervisor, storage, observability.
5) Resolve the agent coordinator (`internal/app/agent/coordinator`).
6) Attach delivery renderers (CLI/TUI, SSE, web).

![Startup Flow Diagram](images/startup_flow.png)

```mermaid
flowchart TD
    A[CLI/TUI | Server | ACP | Web] --> B[internal/shared/config + internal/infra/environment]
    B --> C[internal/app/di Container]
    C --> D[internal/app/agent/coordinator.AgentCoordinator]
    C --> E[internal/infra/llm + internal/app/subscription]
    C --> F[internal/app/toolregistry + internal/infra/tools + internal/infra/mcp]
    C --> G[internal/infra/session + internal/app/context + internal/infra/memory]
    C --> H[internal/infra/storage + internal/infra/observability]
    D --> I[internal/delivery/output + internal/delivery/server]
    I --> J[Terminal | HTTP/SSE | Web UI]
```

---

## 5) Agent Execution Flow (Think -> Act -> Observe)

The agent loop is implemented in the domain layer and orchestrated by the application layer.
Each step emits typed events which are rendered by delivery adapters.

### ReAct Execution Loop

**High-level steps:**
1) **Bootstrap**: Build the system prompt, load session context, capture environment.
2) **Think**: Request reasoning tokens from the configured LLM.
3) **Act**: Parse tool intentions, resolve tool metadata, and dispatch tool execution.
4) **Observe**: Capture tool results, compress logs, update context/memory.
5) **Control**: Apply policies (max steps, approvals, mode restrictions).
6) **Complete**: Emit final response, cost summary, and persistence signals.

### Memory & Sessions

- Session state and transcript persistence live in `internal/infra/session` and `internal/app/context`.
- Retrieval and long-running memory live in Markdown files via `internal/infra/memory` (wired in `internal/app/di`).
- Compression and context windowing happens before each loop iteration to keep prompts bounded.

![Agent Execution Flow Diagram](images/agent_execution_flow.png)

```mermaid
flowchart TD
    A[User Prompt] --> B[Bootstrap: context + env + session]
    B --> C[Think: LLM reasoning tokens]
    C --> D[Act: parse tool calls]
    D --> E{Tool allowed?}
    E -- yes --> F[Execute tool via registry/MCP]
    E -- approval --> G[Approval gate]
    E -- no --> H[Refuse or skip]
    F --> I[Observe: tool output + events]
    G --> F
    I --> J{Stop condition?}
    J -- no --> C
    J -- yes --> K[Complete: response + cost + persist]
    K --> L[Render to CLI/TUI/SSE/Web]
```

---

## 6) Event Stream & Observability

- Domain emits typed events (`internal/domain/agent/events.go`).
- Output adapters stream them to:
  - CLI/TUI (`internal/delivery/output`)
  - HTTP/SSE (`internal/delivery/server`)
  - Web UI (`web/`)
- Observability attaches metrics and traces to LLM calls and tool invocations
  via `internal/infra/observability` and `internal/shared/logging`.

![Event and Observability Pipeline](images/event_pipeline_gen.png)

---

## 7) Execution Modes and Safety

- Session tool modes (full/read-only/safe/sandbox) are enforced via presets in
  `internal/shared/agent/presets`.
- Tool approvals are mediated via `internal/domain/agent/ports` and enforced by the
  tool registry (`internal/app/toolregistry`) with delivery-specific approvers for
  CLI/TUI, web, and ACP.
- Sandbox execution (when enabled) is routed through `internal/infra/sandbox` adapters.

---

## 8) Suggested Reading Order

1) `AGENTS.md` for repo agent workflow, safety rules, and prompt posture.
2) `docs/reference/ARCHITECTURE_AGENT_FLOW.md` (this doc) for the reasoning loop narrative + system map.
3) `docs/reference/CONFIG.md` for configuration and init wiring.
4) `docs/reference/ACP.md` for external client integration.
