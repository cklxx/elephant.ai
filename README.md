<p align="center">
  <img src="assets/banner.png" alt="elephant.ai banner" width="100%" />
</p>

<h1 align="center">elephant.ai</h1>

<p align="center">
  <strong>A leader agent for attention-heavy work.</strong><br/>
  Lives in Lark. Tracks what's moving, surfaces what matters, follows up automatically, and only pulls you in when judgment is needed.
</p>

<p align="center">
  <a href="https://github.com/cklxx/elephant.ai/actions/workflows/ci.yml"><img src="https://github.com/cklxx/elephant.ai/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
  <a href="https://goreportcard.com/report/github.com/cklxx/elephant.ai"><img src="https://goreportcard.com/badge/github.com/cklxx/elephant.ai" alt="Go Report Card"/></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"/></a>
  <a href="README.zh.md"><img src="https://img.shields.io/badge/文档-中文-blue.svg" alt="中文文档"/></a>
</p>

---

## What is elephant.ai?

elephant.ai is a **leader agent** that lives inside your Lark groups and DMs. It doesn't just answer questions — it takes ownership of work: tracking progress, following up on tasks, surfacing what needs your attention, and handling coordination so you don't have to.

Behind the scenes it orchestrates specialist agents and tools. But to you, it feels like having one reliable operator who never drops the ball. CLI and web dashboard are there when you need them, but Lark is home.

---

## ✨ Why a leader agent?

| | |
|---|---|
| 📌 **Continuous ownership** | Hand off a task and it stays tracked. No more "where did this go?" — the leader agent keeps working until it's done. |
| 🔇 **Attention gating** | Compresses noise into what matters now, which risks are growing, and what needs your call. |
| 🔄 **Proactive follow-up** | Chases status, summarizes results, escalates blockers. You don't ask "any updates?" ever again. |
| 🤝 **Coordination absorbed** | The real work tax is track, ask, remind, align. The leader agent handles that overhead. |
| 🧠 **Persistent memory** | Remembers conversations, decisions, and context across weeks and months. Context compounds over time. |
| 🛡️ **Approval gates** | Knows when to ask before acting. Risky operations require explicit human sign-off in chat. |
| 🔌 **MCP extensible** | Connect any external tool through the Model Context Protocol. Infinite integrations. |
| 🏠 **Lark-native** | WebSocket gateway — always present in groups and DMs, no `/slash` commands needed. |

---

## 🚀 Quick Start

**Prerequisites:** Go 1.24+, Node.js 20+, a Lark bot token, and an LLM API key.

```bash
# 1. Clone and build
git clone https://github.com/cklxx/elephant.ai.git && cd elephant.ai
make build

# 2. Configure (LLM key + Lark credentials)
cp examples/config/runtime-config.yaml ~/.alex/config.yaml
export LLM_API_KEY="sk-..."
alex setup   # interactive first-run wizard

# 3. Start everything
alex dev up

# 4. Talk to it in Lark — or use the CLI directly
./alex "summarize the last 3 conversations and draft follow-up emails"
```

Full setup guide → [`docs/guides/quickstart.md`](docs/guides/quickstart.md)

---

## How It Works

```
You (Lark group or DM)
        ↓
  Context Assembly          — chat history + memory + policies
        ↓
  ReAct Agent Loop          — Think → Act → Observe
        ↓
  Tool Execution            — search · code · browser · files · MCP
        ↓
  Reply delivered to Lark   — with live progress and emoji reactions
```

---

## Delivery Surfaces

| Surface | Description |
|---|---|
| **Lark** *(primary)* | WebSocket gateway. Always present in groups/DMs. Real-time tool progress, emoji reactions, approval flows. |
| **Web Console** | Next.js dashboard with SSE streaming, artifact rendering, cost tracking, session history. |
| **CLI / TUI** | Interactive terminal with streaming output. Useful for developers and local workflows. |

---

## Built-in Skills

Skills are markdown-driven workflows triggered by natural language — just describe what you need:

| Skill | What it does |
|---|---|
| `deep-research` | Multi-step web research with source synthesis |
| `meeting-notes` | Structured summaries and action item extraction |
| `email-drafting` | Context-aware email composition |
| `ppt-deck` | Slide deck generation |
| `video-production` | Video script and production planning |
| `research-briefing` | Concise briefing documents from research |
| `best-practice-search` | Engineering best practice lookup |

---

## LLM Providers

```
OpenAI · Anthropic (Claude) · DeepSeek · ByteDance ARK (Doubao)
OpenRouter · Ollama (local) · Kimi · Qwen
```

Set `llm_provider: auto` — the runtime picks the best available subscription from your environment keys automatically.

---

## Architecture

```
Delivery      Lark · Web Console · CLI · API Server
     ↓
Application   Coordination · Context Assembly · Cost Control
     ↓
Domain        ReAct Loop · Typed Events · Approval Gates
     ↓
Infra         Multi-LLM · Memory Store · Tool Registry · Observability
```

| Layer | Key packages |
|---|---|
| Delivery | `internal/delivery/channels/lark/`, `cmd/alex-server`, `web/` |
| Agent core | `internal/{app,domain}/agent` — ReAct loop, typed events, approval gates |
| Tools | `internal/infra/tools/builtin/` — search, code, browser, files, artifacts, media |
| Memory | `internal/infra/memory/` — Postgres / file / in-memory with tokenization |
| LLM | `internal/infra/llm/` — multi-provider, auto-selection, streaming |
| MCP | `internal/infra/mcp/` — JSON-RPC servers for external integrations |
| Observability | `internal/infra/observability/` — OTel traces, Prometheus metrics, cost accounting |

---

## 📖 Documentation

| | |
|---|---|
| [Quick Start](docs/guides/quickstart.md) | From clone to running in minutes |
| [Configuration Reference](docs/reference/CONFIG.md) | Full config schema and precedence rules |
| [Architecture](docs/reference/ARCHITECTURE.md) | Runtime layering and execution model |
| [Deployment Guide](docs/operations/DEPLOYMENT.md) | Production deployment |
| [Roadmap](ROADMAP.md) | What's next |

---

## 🤝 Contributing

Contributions are welcome. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for development setup, code standards, and PR workflow. First time? Look for issues labeled [`good first issue`](https://github.com/cklxx/elephant.ai/issues?q=label%3A%22good+first+issue%22).

Please read [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) before participating, and report security vulnerabilities via [`SECURITY.md`](SECURITY.md).

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=cklxx/elephant.ai&type=Date)](https://star-history.com/#cklxx/elephant.ai&Date)

---

## License

[MIT](LICENSE) © 2025 cklxx
