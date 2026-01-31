<p align="center">
    <img src="web/public/elephant-rounded.png" alt="elephant.ai mascot" width="76" height="76" />
</p>

# elephant.ai

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**A proactive personal AI — your own dedicated AI agent.**

elephant.ai is not another chatbot you have to ask. It is a proactive personal AI agent that embeds into your daily workflows — Lark, CLI, and web — remembers context across conversations, takes initiative with built-in skills, and executes real work autonomously. One runtime, every surface, always ready.

---

## What makes it proactive

| Capability | What it does |
|---|---|
| **Persistent memory** | Remembers conversations, decisions, and context across sessions. Retrieves relevant history automatically so you never repeat yourself. |
| **Channel-native** | Lives inside Lark as a first-class participant. Reads the room, responds in-thread, and acts on messages in real time. |
| **Autonomous execution** | Runs a full Think → Act → Observe loop. Searches the web, writes code, generates documents, and browses pages without hand-holding. |
| **Built-in skills** | Ships with ready-to-use workflows: deep research, meeting notes, email drafting, slide decks, video production, and more. |
| **Context-aware reasoning** | Assembles system policies, task goals, memory, and session history into a layered context — then reasons with extended thinking models (Claude, OpenAI o-series, DeepSeek). |
| **Approval gates** | Knows when to ask before acting. Risky operations require explicit human approval via CLI, web, or chat. |

---

## How it works

```
You (Lark / CLI / Web)
        ↓
   elephant.ai runtime
        ↓
  ┌─────────────────────────────────┐
  │  Context Assembly               │
  │  (memory + history + policies)  │
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
  Response delivered to your channel
```

All surfaces share the same execution core, typed event stream, and observability pipeline. A conversation started in Lark renders identically in the web dashboard.

---

## Channels & surfaces

- **Lark** — WebSocket gateway, auto-saves messages to memory, injects recent chat history as context, real-time tool progress display, emoji reactions, group and DM support.
- **CLI / TUI** — Interactive terminal with streaming output and tool approval prompts.
- **Web dashboard** — Next.js app with SSE streaming, artifact rendering, cost tracking, and session management.

---

## Built-in skills

Skills are markdown-driven workflows the assistant can execute on demand:

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
export OPENAI_API_KEY="sk-..."
# or: ANTHROPIC_API_KEY, CLAUDE_CODE_OAUTH_TOKEN, CODEX_API_KEY, ANTIGRAVITY_API_KEY
cp examples/config/runtime-config.yaml ~/.alex/config.yaml

# 2. Run backend + web together
./dev.sh

# 3. Or build the CLI
make build
./alex
./alex "summarize the last 3 Lark conversations and draft follow-up emails"
```

```bash
# Dev commands
./dev.sh status    # check service status
./dev.sh logs server
./dev.sh logs web
./dev.sh down      # stop services
```

Configuration reference: [`docs/reference/CONFIG.md`](docs/reference/CONFIG.md)

---

## Architecture

```
Delivery (CLI, Server, Web, Lark)
  → Agent Application Layer
  → Domain Ports (ReAct loop, events, approvals)
  → Infrastructure Adapters (LLM, tools, memory, storage, observability)
```

| Layer | Key packages |
|---|---|
| Delivery | `cmd/alex`, `cmd/alex-server`, `web/`, `internal/channels/` |
| Agent core | `internal/agent/{app,domain,ports}` — ReAct loop, typed events, approval gates |
| Tools | `internal/tools/builtin/` — search, code execution, browser, files, artifacts, media |
| Memory | `internal/memory/` — persistent store (Postgres, file, in-memory) with tokenization |
| Context | `internal/context/`, `internal/rag/` — layered retrieval and summarization |
| LLM | `internal/llm/` — multi-provider with auto-selection and streaming |
| MCP | `internal/mcp/` — JSON-RPC tool servers for external integrations |
| Observability | `internal/observability/` — OpenTelemetry traces, Prometheus metrics, cost accounting |
| Storage | `internal/storage/`, `internal/session/` — session persistence and history |
| DI | `internal/di/` — shared dependency injection across all surfaces |

---

## Tools the assistant can use

The assistant has access to a rich set of built-in tools:

- **Web search & browsing** — Search engines and full browser automation via ChromeDP
- **Code execution** — Sandboxed code runner for multiple languages
- **File operations** — Read, write, and manage files
- **Artifact generation** — PDFs, images, and structured outputs
- **Media processing** — Image, audio, and video handling
- **Lark integration** — Send messages, fetch chat history, manage conversations
- **Memory management** — Store and recall information across sessions
- **MCP servers** — Connect any external tool via the Model Context Protocol

---

## Quality & operations

```bash
# Lint & test
./dev.sh lint
./dev.sh test
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
