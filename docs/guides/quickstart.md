# elephant.ai Quickstart

> Last updated: 2026-03-10

## Prerequisites

- Go **1.24+**, Node.js **20+** (web UI only), Docker (optional)

## Configure

```bash
export LLM_API_KEY="sk-..."
cp examples/config/runtime-config.yaml ~/.alex/config.yaml
```

Provider-specific keys (higher priority than `LLM_API_KEY`): `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, `CODEX_API_KEY`, `ANTIGRAVITY_API_KEY`.

Provider examples (`~/.alex/config.yaml`):

```yaml
runtime:
  llm_provider: "auto"           # auto-detect from env keys
  # llm_provider: "anthropic"    # Claude
  # llm_provider: "openai-responses"  # Codex
  # llm_provider: "cli"          # prefer CLI subscriptions
```

Managed overrides:

```bash
alex config set llm_model gpt-4o-mini
alex config clear llm_vision_model
```

## Build and Run

```bash
make build
alex setup                    # first-run provider/model picker
alex dev up                   # start all services
```

```bash
alex dev status               # show status (PID, health, port)
alex dev logs server          # tail server logs
alex dev down                 # stop all services
alex dev restart [service]    # restart one or all
alex dev test                 # Go tests with race + coverage
alex dev lint                 # Go + web lint
```

## Demo

```bash
./alex "Map the runtime layers, explain the event stream, and produce a short summary."
```

- CLI shows typed event stream (plan/tool/output).
- Web UI at `http://localhost:3000` mirrors events and artifacts.

## Sessions

```bash
./alex sessions
./alex sessions pull <session-id>
./alex sessions cleanup --older-than 30d --keep-latest 20 --dry-run
```

## Tool Safety

```bash
alex config set tool_preset safe       # safe | read-only | full | sandbox
```

## Lark Supervisor

```bash
alex dev lark supervise       # foreground supervisor
alex dev lark start           # background daemon
alex dev lark stop
alex dev lark status
```

## Next Steps

- [Configuration Reference](../reference/CONFIG.md)
- [Architecture](../reference/ARCHITECTURE.md)
- [Deployment](../operations/DEPLOYMENT.md)
