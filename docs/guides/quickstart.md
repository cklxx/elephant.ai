# elephant.ai Quickstart
> Last updated: 2025-12-14

This guide gets you from `git clone` to running the `alex` CLI/TUI, server, and web UI. Configure an OpenAI-compatible LLM provider via the shared runtime config system (`docs/reference/CONFIG.md`).

---

## Prerequisites

- Go **1.24+** (pinned in `go.mod`)
- Node.js **20+** (only needed for the web UI)
- Docker (optional, for Compose/K8s workflows)

---

## Configure

```bash
export OPENAI_API_KEY="sk-..."
# export ANTHROPIC_API_KEY="sk-ant-..."   # Claude
# export CLAUDE_CODE_OAUTH_TOKEN="..."    # Claude Code OAuth
# export CODEX_API_KEY="sk-..."           # OpenAI Responses / Codex
# export ANTIGRAVITY_API_KEY="..."        # Antigravity (OpenAI-compatible)
cp examples/config/runtime-config.yaml ~/.alex/config.yaml
```

Provider switches (edit `~/.alex/config.yaml`):

```yaml
runtime:
  llm_provider: "anthropic"        # claude-style API
  llm_model: "claude-3-5-sonnet"
```

```yaml
runtime:
  llm_provider: "openai-responses" # OpenAI Responses / Codex
  llm_model: "gpt-4.1-mini"
```

```yaml
runtime:
  llm_provider: "auto"             # Claude/Codex/Antigravity/OpenAI via env keys
```

```yaml
runtime:
  llm_provider: "cli"              # Prefer CLI subscriptions, fallback to env
```

Optional managed overrides (persistent):

```bash
./alex config
./alex config set llm_model gpt-4o-mini
./alex config set llm_vision_model gpt-4o-mini
./alex config clear llm_vision_model
```

Reference: `docs/reference/CONFIG.md`

---

## Run (recommended)

`dev.sh` builds and runs the backend + web UI together.

```bash
./dev.sh
```

Check status or logs:

```bash
./dev.sh status
./dev.sh logs server
./dev.sh logs web
```

Stop services:

```bash
./dev.sh down
```

## Demo (first run)

```bash
make build
./alex "Map the runtime layers, explain the event stream, and produce a short summary."
```

Expected:

- CLI shows a typed event stream (plan/tool/output).
- Web UI at `http://localhost:3000` mirrors the same events and artifacts.
- Cost + trace metadata updates with each run.

### CLI / TUI (optional)

```bash
make build
./alex
./alex "analyze the authentication flow and list risks"
```

---

## Sessions

```bash
./alex sessions
./alex sessions pull <session-id>
./alex sessions cleanup --older-than 30d --keep-latest 20 --dry-run
```

---

## Tool safety (recommended)

Use `tool_preset` to control which tools the CLI agent can call (web mode ignores it and enables all non-local tools):

```bash
./alex config set tool_preset safe
./alex config set tool_preset read-only
./alex config set tool_preset full
```

---

## Validate

```bash
./dev.sh lint
./dev.sh test
```

---

## Next steps

- Deployment: `docs/operations/DEPLOYMENT.md`
- Configuration reference: `docs/reference/CONFIG.md`
- Runtime/agent deep dive: `docs/AGENT.md`
