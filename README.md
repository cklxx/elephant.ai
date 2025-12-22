# elephant.ai

elephant.ai provides a Go backend and Next.js dashboard built around a shared Think → Act → Observe loop. The `alex` CLI/TUI, HTTP + SSE server, and web UI run the same runtime so operators and automation stay in sync.

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## What it does

* **Fragment-to-plan reasoning.** Streams shell transcripts, notebook outputs, tickets, and freeform notes into stitched execution plans using the shared agent runtime.
* **One runtime, many surfaces.** CLI/TUI (`cmd/alex`), server (`cmd/alex-server`), and dashboard (`web/`) all share the same dependency injection container and event stream.
* **Tooling that respects context.** File, shell, search, and notebook helpers emit typed events with artifacts/attachments for the dashboard.
* **Observability-first.** Structured logs, OpenTelemetry traces, Prometheus metrics, and per-session cost tracking live under `internal/observability`.
* **Evaluation harnesses.** SWE-Bench runners in `evaluation/` keep quality measurable during development.

---

## Architecture at a glance

```
Delivery (CLI, Server, Web) → Agent Application Layer → Domain Ports → Infrastructure Adapters
```

| Layer | Highlights |
| --- | --- |
| Delivery | `cmd/alex`, `cmd/alex-server`, and `web/` consume the same DI container for consistent behavior. |
| Agent core | `internal/agent/app`, `internal/agent/domain`, and `internal/agent/ports` implement the ReAct loop, approvals, and typed events. |
| Infrastructure | `internal/di`, `internal/tools`, `internal/toolregistry`, `internal/llm`, `internal/mcp`, `internal/session`, `internal/storage`, `internal/observability`, and `internal/context` provide adapters, LLM clients, MCP runtime, persistence, rendering, telemetry, and layered context. |
| Frontend | `web/` renders real-time sessions via SSE, supports cost inspection, and lets operators feed new fragments. |

More detail lives in [`docs/AGENT.md`](docs/AGENT.md). New to the codebase? Start with [`READMAP.md`](READMAP.md).

---

## Repository map

| Path | Purpose |
| --- | --- |
| `cmd/` | Go entrypoints (`alex`, `alex-server`). |
| `internal/` | Agent core, DI, tools, context orchestration, storage, and observability. |
| `web/` | Next.js dashboard with SSE client, session list, and controls. |
| `evaluation/` | SWE-Bench automation and reporting utilities. |
| `docs/` | Architecture notes, references, operations guides, and research. |
| `tests/` | End-to-end and integration suites executed in CI. |
| `scripts/` | Developer automation and CI helpers. |
| `deploy/docker/` | Dockerfiles, Compose stacks, and nginx config for containerized deployments. |

---

## Quickstart

### Prerequisites

* Go **1.24+** (see `go.mod`).
* Node.js **20+** for the dashboard.
* Docker (optional) for containerized deployments.

### CLI / TUI

```bash
make build        # build ./alex
./alex            # launch the TUI
./alex --no-tui   # run in line-mode
```

Common maintenance commands:

```bash
./alex sessions cleanup --older-than 30d --keep-latest 25
./alex sessions cleanup --older-than 14d --dry-run
```

### Server + Dashboard

```bash
./alex-server            # start HTTP + SSE server on :8080
(cd web && npm install)  # install frontend deps
(cd web && npm run dev)  # launch Next.js dashboard
```

For split origins during development, set `NEXT_PUBLIC_API_URL=http://localhost:8080` (or keep the default `auto` for same-origin setups).

### Production

1. Configure secrets and models in `~/.alex-config.json` (override via `ALEX_CONFIG_PATH`) and export keys such as `OPENAI_API_KEY` and `TAVILY_API_KEY`. See [`docs/reference/CONFIG.md`](docs/reference/CONFIG.md).
2. Run `./deploy.sh` for the nginx-backed stack on port 80. Mainland China networks can use `./deploy.sh cn` for mirror settings. Inspect with `./deploy.sh pro status|logs|down`.
3. For Compose-based production, prefer `deploy/docker/docker-compose.yml`:
   ```bash
   echo "OPENAI_API_KEY=sk-your-key" > .env
   docker compose -f deploy/docker/docker-compose.yml up -d
   docker compose -f deploy/docker/docker-compose.yml logs -f alex-server
   ```
4. Wire liveness probes to `/health` and ship logs/metrics to your monitoring stack.

---

## Development workflow

```bash
make fmt     # golangci-lint (fix) + format
make vet     # go vet on cmd/ and internal/
make build   # compile CLI
make test    # run all Go tests
```

Frontend commands live in `web/README.md`; evaluation jobs use scripts in `evaluation/`.

---

## Skills (Markdown playbooks)

Reusable playbooks in `skills/` are indexed into the context builder and discoverable via the `skills` tool.

* Each skill is a `.md/.mdx` file with YAML front matter:
  ```md
  ---
  name: my_skill
  description: One-line summary used for discovery/tooling.
  ---
  # Title
  ...
  ```
* Override the skills directory with `ALEX_SKILLS_DIR=/path/to/skills`.

Built-in examples include `video_production` and `ppt_deck`.

---

## Roadmap snapshots

* **TUI polish.** Better inline help, command palette hints, and transcript exports.
* **Server hardening.** Expanded health/readiness probes plus structured error responses; K8s manifests are available today.
* **Memory accuracy.** Retrieval and summarization tuning within `internal/context` and `internal/rag`.
* **Tool safety.** Approval policies for destructive actions with configurable templates.
* **Evaluation coverage.** Growing SWE-Bench and regression suites under `evaluation/` and `tests/`.

---

## Documentation index

* [`READMAP.md`](READMAP.md) – guided reading order for the codebase.
* [`docs/README.md`](docs/README.md) – documentation index.
* [`docs/AGENT.md`](docs/AGENT.md) – orchestration flow and event model.
* [`docs/reference/CONFIG.md`](docs/reference/CONFIG.md) – configuration schema and precedence.
