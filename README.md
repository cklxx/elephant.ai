<p align="center">
    <img src="web/public/elephant-rounded.png" alt="elephant.ai mascot" width="76" height="76" />
</p>

# elephant.ai

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Lark-native proactive personal agent.**

[中文文档](README.zh.md)

elephant.ai lives inside your Lark groups and DMs as a first-class participant — not a bot you have to summon. It reads the room, remembers context across conversations, takes initiative with built-in skills, and executes real work autonomously. CLI and web dashboard are there when you need them, but Lark is home.

---

## Why Lark-native

Most AI assistants sit outside your workflow — a separate app, a separate tab, a separate context switch. elephant.ai is different:

| Capability | How it works in Lark |
|---|---|
| **Always present** | Lives in your Lark groups and DMs via WebSocket. No `/slash` commands — just talk naturally. |
| **Reads the room** | Auto-fetches recent chat history as context. Understands the conversation before replying. |
| **Persistent memory** | Remembers conversations, decisions, and context across sessions. Never repeat yourself. |
| **Autonomous execution** | Full Think → Act → Observe loop. Web search, code, documents, browser — all from a Lark message. |
| **Live progress** | Shows tool execution progress and emoji reactions in real time while working. |
| **Built-in skills** | Deep research, meeting notes, email drafting, slide decks, video production — triggered by natural language. |
| **Approval gates** | Knows when to ask before acting. Risky operations require explicit human approval right in the chat. |

---

## North Star: Calendar + Tasks closed loop (M0)

The core vertical slice lives entirely in Lark: **read calendar/tasks → propose actions → write with approval → proactively follow up**.

Implemented building blocks:
- **Calendar tools:** query/create/update/delete events (`lark_calendar_*`)
- **Task tools:** list/create/update/delete tasks (`lark_task_manage`)
- **Proactive reminders:** scheduler trigger that checks upcoming events/tasks and messages you in Lark

See `docs/roadmap/roadmap.md` for the current M0/M1 status and `docs/reference/CONFIG.md` for config details.

---

## How it works

```
You (Lark group / DM)
        ↓
   elephant.ai runtime
        ↓
  ┌─────────────────────────────────┐
  │  Context Assembly               │
  │  (chat history + memory +       │
  │   policies + session state)     │
  ├─────────────────────────────────┤
  │  ReAct Agent Loop               │
  │  Think → Act → Observe          │
  ├─────────────────────────────────┤
  │  Tool Execution                 │
  │  (search, code, browse, files,  │
  │   artifacts, MCP servers)       │
  ├─────────────────────────────────┤
  │  Observability                  │
  │  (traces, metrics, cost)        │
  └─────────────────────────────────┘
        ↓
  Reply delivered back to Lark
```

---

## Surfaces

- **Lark** (primary) — WebSocket gateway, auto-saves messages to memory, injects recent chat history as context, real-time tool progress, emoji reactions, group and DM support, plan review and approval flows.
- **Web dashboard** — Next.js app with SSE streaming, artifact rendering, cost tracking, session management. Useful for reviewing past conversations and complex outputs.
- **CLI / TUI** — Interactive terminal with streaming output and tool approval prompts. For developers who prefer the command line.

---

## Built-in skills

Skills are markdown-driven workflows the agent executes on demand — just describe what you need in Lark:

| Skill | Description |
|---|---|
| `deep-research` | Multi-step web research with source synthesis |
| `meeting-notes` | Structured meeting summaries and action items |
| `email-drafting` | Context-aware email composition |
| `ppt-deck` | Slide deck generation |
| `video-production` | Video script and production planning |
| `research-briefing` | Concise briefing documents from research |
| `best-practice-search` | Engineering best practice lookup |

---

## LLM providers

elephant.ai supports multiple providers and picks the best available one automatically:

- **OpenAI** — Chat API + Responses API (GPT-4o, o-series)
- **Anthropic** — Claude API (Claude 3.5/4 family, extended thinking)
- **ByteDance ARK** — With reasoning effort control
- **DeepSeek** — DeepSeek models via OpenAI-compatible gateway
- **OpenRouter** — Access to 100+ models
- **Ollama** — Local models, zero cloud dependency
- **Antigravity** — OpenAI-compatible gateway

Set `llm_provider: auto` and the runtime resolves the best available subscription from your CLI auth and environment keys.

---

## Getting started

Prerequisites: Go 1.24+, Node.js 20+ (web UI), Docker (optional).

```bash
# 1. Configure your LLM provider
export LLM_API_KEY="sk-..."
# optional provider-specific overrides: OPENAI_API_KEY, ANTHROPIC_API_KEY, CLAUDE_CODE_OAUTH_TOKEN, CODEX_API_KEY, ANTIGRAVITY_API_KEY
cp examples/config/runtime-config.yaml ~/.alex/config.yaml
alex config validate --profile quickstart

# 2. Configure Lark bot credentials in ~/.alex/config.yaml
#    channels:
#      lark:
#        enabled: true
#        app_id: "cli_xxx"
#        app_secret: "xxx"
#        tenant_calendar_id: "cal_xxx" # shared calendar for tenant token fallback
#        persistence:
#          mode: "file"      # file|memory
#          dir: "~/.alex/lark"
#
# Optional: enable proactive calendar/task reminders (scheduler)
#    runtime:
#      proactive:
#        scheduler:
#          enabled: true
#          calendar_reminder:
#            enabled: true
#            schedule: "*/15 * * * *"
#            look_ahead_minutes: 120
#            channel: "lark"
#            user_id: "ou_xxx"
#            chat_id: "oc_xxx"

# 3. Build and run services (sandbox, backend, web)
make build
alex setup                     # first-run wizard (runtime + lark + model)
alex dev up

# 4. Or use the CLI directly
./alex
./alex "summarize the last 3 Lark conversations and draft follow-up emails"
```

Configuration reference: [`docs/reference/CONFIG.md`](docs/reference/CONFIG.md)

---

## Development (`alex dev`)

All dev environment management is built into the `alex` binary — no shell scripts needed.

```bash
make build                     # build the alex binary

# Service lifecycle
alex dev up                    # start services (sandbox, backend, web)
alex dev up --lark             # lark mode: skips auth DB by default
alex dev up --lark --with-authdb  # lark mode with explicit auth DB
alex dev down                  # stop all services gracefully
alex dev status                # show status of each service (PID, health, port)
alex dev restart [service]     # restart one or all services
alex dev logs [service]        # tail logs (server|web|all)

# Sandbox management
alex dev sandbox up            # start sandbox container only
alex dev sandbox down          # stop sandbox container
alex dev sandbox status        # check sandbox health

# Quality
alex dev test                  # run Go tests (race + coverage)
alex dev lint                  # run Go + web lint
alex setup                     # first-run setup wizard

# Lark supervisor (production)
alex dev lark                  # default: start supervisor daemon
alex dev lark supervise        # foreground supervisor with restart policy
alex dev lark up|start         # background supervisor daemon
alex dev lark down|stop        # stop supervisor
alex dev lark restart          # restart supervisor
alex dev lark status           # show supervisor health + components

# Log analyzer UI
alex dev logs-ui               # start services and open log analyzer in browser
```

Key improvements over the legacy `./dev.sh`:
- **Race-free port allocation** — reserves ports via `net.Listen` before service startup
- **PID double-check** — verifies processes via `kill -0`, not just PID files
- **PGID-based cleanup** — kills entire process groups on shutdown, no orphans
- **Atomic state files** — tmp + rename for PID and supervisor state
- **Restart storm detection** — time-windowed history with exponential backoff
- **Typed config** — Go structs replace awk-parsed YAML; defaults + env + YAML layered

---

## Architecture

```
Delivery (Lark, Web, CLI)
  → Agent Application Layer
  → Domain Ports (ReAct loop, events, approvals)
  → Infrastructure Adapters (LLM, tools, memory, storage, observability)
```

| Layer | Key packages |
|---|---|
| Delivery | `internal/delivery/channels/lark/`, `cmd/alex-server`, `web/`, `cmd/alex` |
| Agent core | `internal/{app,domain}/agent` — ReAct loop, typed events, approval gates |
| Tools | `internal/infra/tools/builtin/` — search, code execution, browser, files, artifacts, media |
| Memory | `internal/infra/memory/` — persistent store (Postgres, file, in-memory) with tokenization |
| Context | `internal/app/context/` — layered context selection and summarization |
| LLM | `internal/infra/llm/` — multi-provider with auto-selection and streaming |
| MCP | `internal/infra/mcp/` — JSON-RPC tool servers for external integrations |
| Observability | `internal/infra/observability/` — OpenTelemetry traces, Prometheus metrics, cost accounting |
| Storage | `internal/infra/storage/`, `internal/infra/session/` — session persistence and history |
| DI | `internal/app/di/` — shared dependency injection across all surfaces |

---

## Tools the agent can use

- **Web search & browsing** — Search engines and full browser automation via ChromeDP
- **Code execution** — Sandboxed code runner for multiple languages
- **File operations** — Read, write, and manage files
- **Artifact generation** — PDFs, images, and structured outputs
- **Media processing** — Image, audio, and video handling
- **Lark integration** — Send messages, fetch chat history, and manage Calendar/Tasks
- **Memory management** — Store and recall information across sessions
- **MCP servers** — Connect any external tool via the Model Context Protocol

---

## Quality & operations

```bash
# Lint & test
alex dev lint
alex dev test
npm --prefix web run e2e

# Evaluation harnesses (SWE-Bench, regressions)
# See evaluation/ directory
```

Observability stack: structured logs, OpenTelemetry traces, Prometheus metrics, and per-session cost accounting — all built in.

---

## Docs

| Document | Description |
|---|---|
| [`docs/README.md`](docs/README.md) | Documentation landing page |
| [`docs/reference/ARCHITECTURE_AGENT_FLOW.md`](docs/reference/ARCHITECTURE_AGENT_FLOW.md) | Architecture and execution flow |
| [`docs/reference/CONFIG.md`](docs/reference/CONFIG.md) | Configuration schema and precedence |
| [`docs/guides/quickstart.md`](docs/guides/quickstart.md) | From clone to running |
| [`docs/operations/DEPLOYMENT.md`](docs/operations/DEPLOYMENT.md) | Deployment guide |
| [`AGENTS.md`](AGENTS.md) | Agent workflow and safety rules |
| [`ROADMAP.md`](ROADMAP.md) | Roadmap and contribution queue |

---

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for workflow and code standards, [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) for community expectations, and [`SECURITY.md`](SECURITY.md) for vulnerability reporting.

Licensed under [MIT](LICENSE).
